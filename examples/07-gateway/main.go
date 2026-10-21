// A gateway polls a temperature probe every 10ms, by an rrule set whose X-FREQ is PT0.01S, and fans each reading out
// to a local store and a cloud API with multicast.
//
// fn runs in Ring's goroutine, and only reads the probe and publishes: both subscribers are buffered and never waited
// for, so that neither a slow disk nor a slow network delays a poll. The local store saves every reading, a
// transaction per 10. The cloud gets a reading once it has moved by half a degree, or a minute after the last however
// steady, in a request every 10s, retried with backoff while the API is down. Once the polls end, Shutdown hands both
// what they hold.
//
// Device, LocalStore and Cloud stand for a Modbus client, a database and an HTTP client, which fake.go fakes in
// process. It is a module of its own, to keep rrule and multicast out of the library's go.mod:
// `go run -C examples/07-gateway .`.
package main

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log"
	"slices"
	"time"

	"github.com/elojah/multicast"
	"github.com/elojah/multicast/filter"
	"github.com/elojah/multicast/holder"
	"github.com/elojah/multicast/message"
	mcmiddleware "github.com/elojah/multicast/middleware"
	"github.com/elojah/multicast/subscriber"
	"github.com/elojah/rrule"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/middleware"
)

const (
	// schedule polls every 10ms: X-FREQ holds the period, and FREQ=SECONDLY is for readers that do not know it. A
	// gateway would read it from its configuration.
	schedule = "DTSTART:20260101T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.01S"
	// A gateway would poll until SIGTERM, with the ctx signal.NotifyContext returns and no end. Here, 30 times.
	polls = 30

	// buffer is a second of polls, which a subscriber may lag behind before it loses readings.
	buffer = 100
	// The local store saves a transaction per localBatch readings, or per localWindow while polls fail.
	localBatch  = 10
	localWindow = time.Second
	// The cloud gets a reading once it has moved by deadband, in tenths of a degree, or once heartbeat has gone by, in
	// a request per cloudWindow, or per cloudBatch readings.
	deadband    = 5
	heartbeat   = time.Minute
	cloudWindow = 10 * time.Second
	cloudBatch  = 100
	// shutdownTimeout bounds how long the subscribers take with what they hold once the polls end.
	shutdownTimeout = 5 * time.Second
)

// Device is the probe, such as a Modbus client. Read returns within a timeout of its own, so that a probe gone quiet
// skips polls rather than delays them.
type Device interface {
	Read(ctx context.Context) (tenths, error)
}

// LocalStore keeps every reading on the gateway, such as a SQLite database.
type LocalStore interface {
	Save(ctx context.Context, readings []reading) error
}

// Cloud is an API that takes readings, such as over HTTPS.
type Cloud interface {
	Send(ctx context.Context, readings []reading) error
}

// tenths is a temperature in tenths of a degree, as a Modbus register holds it.
type tenths int16

func (t tenths) String() string {
	return fmt.Sprintf("%.1f°C", float64(t)/10)
}

// reading is a temperature at a time of the schedule.
type reading struct {
	At          time.Time
	Temperature tenths
}

// gateway polls its device, and fans each reading out to its local store and the cloud.
type gateway struct {
	device Device
	local  LocalStore
	cloud  Cloud
}

func main() {
	set, err := rrule.ParseSet(schedule)
	if err != nil {
		log.Fatal(err)
	}
	every := set.RRules[0].XFreq

	device, local, cloud := &probe{}, &disk{}, &api{}
	gw := gateway{device: device, local: local, cloud: cloud}
	now := time.Now()
	if err := gw.run(context.Background(), set.Between(now, now.Add(polls*every)), every); err != nil {
		log.Fatal(err)
	}

	for _, line := range slices.Concat(local.lines, cloud.lines) {
		fmt.Println(line)
	}
}

// run polls the device at each of times, until they end or ctx is done, dropping a poll later than late. Then it
// hands the local store and the cloud what they hold, and returns once they are done with it.
func (gw gateway) run(ctx context.Context, times iter.Seq[time.Time], late time.Duration) error {
	// The subscribers outlive ctx, so that Shutdown still hands them what they hold once it is done.
	subscribed := context.WithoutCancel(ctx)
	g := multicast.New(multicast.WithDefaultSubscriberOptions(func() []subscriber.Option[reading] {
		return []subscriber.Option[reading]{
			// Publish never waits: a reading that finds the buffer full is lost, as a *subscriber.DroppedError.
			subscriber.WithDefaultMessageOptions(message.WithNonBlocking[reading]()),
			subscriber.WithBuffer[reading](buffer),
			subscriber.WithErrorHandler[reading](logError),
		}
	}))
	if err := gw.subscribeLocal(subscribed, g); err != nil {
		return err
	}
	if err := gw.subscribeCloud(subscribed, g); err != nil {
		return err
	}

	// No holder.NewSpin: a poll a millisecond late is fine, a core spinning on a gateway is not. MaxDelay drops the
	// polls missed while the gateway was suspended or stalled, which would otherwise read the probe all at once.
	a := alarm.New(alarm.WithMiddleware(middleware.MaxDelay(late)))
	err := a.Ring(ctx, times, func(ctx context.Context, at time.Time) {
		t, err := gw.device.Read(ctx)
		if err != nil {
			// The next poll brings a newer reading.
			log.Println(err)

			return
		}
		// Stamped with the schedule's time, so that readings are evenly spaced, and line up across gateways.
		g.Publish(ctx, reading{At: at, Temperature: t})
	})

	shutdownCtx, cancel := context.WithTimeout(subscribed, shutdownTimeout)
	defer cancel()

	return errors.Join(err, g.Shutdown(shutdownCtx))
}

// subscribeLocal saves every reading, a transaction per localBatch.
func (gw gateway) subscribeLocal(ctx context.Context, g *multicast.Group[reading]) error {
	save := func(ctx context.Context, _ uint64, batch message.Batch[reading]) error {
		return gw.local.Save(ctx, slices.Collect(batch.All()))
	}
	// handle gets nothing: the holder releases batches only.
	_, err := g.Subscribe(ctx, func(ctx context.Context, _ uint64, r reading) error {
		return gw.local.Save(ctx, []reading{r})
	},
		subscriber.WithHolder(holder.NewBatch[reading](localWindow, localBatch)),
		subscriber.WithBatchHandler(save),
	)

	return err
}

// subscribeCloud sends a reading once it has moved by deadband, or once heartbeat has gone by, a request per
// cloudWindow. A request the API still refuses after the retries is lost: the local store has the readings.
func (gw gateway) subscribeCloud(ctx context.Context, g *multicast.Group[reading]) error {
	send := func(ctx context.Context, _ uint64, batch message.Batch[reading]) error {
		return gw.cloud.Send(ctx, slices.Collect(batch.All()))
	}
	// With jitter, so that gateways that failed together retry apart.
	retry := mcmiddleware.Retry[message.Batch[reading]](mcmiddleware.RetryPolicy{
		Attempts: 5, Delay: 100 * time.Millisecond, Multiplier: 2, Jitter: 0.2,
	})
	// handle gets nothing: the holder releases batches only.
	_, err := g.Subscribe(ctx, func(ctx context.Context, _ uint64, r reading) error {
		return gw.cloud.Send(ctx, []reading{r})
	},
		subscriber.WithFilter(filter.Changed(func(prev, next reading) bool {
			return max(prev.Temperature, next.Temperature)-min(prev.Temperature, next.Temperature) >= deadband
		}, filter.WithHeartbeat[reading](heartbeat))),
		subscriber.WithHolder(holder.NewBatch[reading](cloudWindow, cloudBatch)),
		subscriber.WithBatchHandler(send, retry),
	)

	return err
}

func logError(_ context.Context, err error) {
	log.Println(err)
}
