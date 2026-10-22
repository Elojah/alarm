// A gateway samples a pump's vibration every 100ms while the pump idles, and every 10ms while it runs, by two rrule
// sets whose X-FREQs are PT0.1S and PT0.01S. Once a sample shows that the pump started or stopped, fn Resets the alarm
// to the other set's times after it, so that the next sample comes at the new rate. Both sets repeat from a whole
// second, so that either rate picks up on the same grid.
//
// Each sample fans out with multicast to a local store, which saves every one, a transaction per 10, and to the cloud,
// which gets each start and stop. Device, LocalStore and Cloud stand for a vibration sensor, a database and an HTTP
// client, which fake.go fakes in process. It is a module of its own, to keep rrule and multicast out of the library's
// go.mod: `go run -C examples/08-adaptive .`.
package main

import (
	"context"
	"errors"
	"fmt"
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
	// The pump is sampled every 100ms while it idles, and every 10ms while it runs. A gateway would read the
	// schedules from its configuration.
	idleSchedule    = "DTSTART:20260101T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.1S"
	runningSchedule = "DTSTART:20260101T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.01S"
	// A gateway would sample until SIGTERM, with the ctx signal.NotifyContext returns and no end. Here, for 600ms.
	window = 600 * time.Millisecond

	// The pump starts at a vibration of startAt, in mm/s, and stops below stopAt, so that a vibration around either
	// does not switch the rate at each sample.
	startAt = 2.0
	stopAt  = 1.0

	// buffer is a second of samples while the pump runs, which a subscriber may lag behind before it loses samples.
	buffer = 100
	// The local store saves a transaction per localBatch samples, or per localWindow while the pump idles.
	localBatch  = 10
	localWindow = time.Second
	// shutdownTimeout bounds how long the subscribers take with what they hold once the samples end.
	shutdownTimeout = 5 * time.Second
)

// Device is the vibration sensor. Read returns the vibration's velocity, RMS in mm/s, within a timeout of its own, so
// that a sensor gone quiet skips samples rather than delays them.
type Device interface {
	Read(ctx context.Context) (float64, error)
}

// LocalStore keeps every sample on the gateway, such as a SQLite database.
type LocalStore interface {
	Save(ctx context.Context, samples []sample) error
}

// Cloud is an API that takes the pump's starts and stops, such as over HTTPS.
type Cloud interface {
	Send(ctx context.Context, s sample) error
}

// sample is the pump's vibration at a time of a schedule, and whether it runs.
type sample struct {
	At       time.Time
	Velocity float64
	Running  bool
}

func (s sample) String() string {
	state := "idle"
	if s.Running {
		state = "running"
	}

	return fmt.Sprintf("%.1f mm/s, %s", s.Velocity, state)
}

// gateway samples its device, and fans each sample out to its local store and the cloud.
type gateway struct {
	device Device
	local  LocalStore
	cloud  Cloud
}

func main() {
	idle, err := rrule.ParseSet(idleSchedule)
	if err != nil {
		log.Fatal(err)
	}
	running, err := rrule.ParseSet(runningSchedule)
	if err != nil {
		log.Fatal(err)
	}

	device, local, cloud := &sensor{}, &disk{}, &api{}
	gw := gateway{device: device, local: local, cloud: cloud}
	now := time.Now()
	if err := gw.run(context.Background(), idle, running, now, now.Add(window)); err != nil {
		log.Fatal(err)
	}

	for _, line := range slices.Concat(local.lines, cloud.lines) {
		fmt.Println(line)
	}
}

// run samples the device at idle's times from now, and running's once the pump starts, until end or until ctx is
// done. Then it hands the local store and the cloud what they hold, and returns once they are done with it.
func (gw gateway) run(ctx context.Context, idle, running rrule.Set, now, end time.Time) error {
	// The subscribers outlive ctx, so that Shutdown still hands them what they hold once it is done.
	subscribed := context.WithoutCancel(ctx)
	g := multicast.New(multicast.WithDefaultSubscriberOptions(func() []subscriber.Option[sample] {
		return []subscriber.Option[sample]{
			// Publish never waits: a sample that finds the buffer full is lost, as a *subscriber.DroppedError.
			subscriber.WithDefaultMessageOptions(message.WithNonBlocking[sample]()),
			subscriber.WithBuffer[sample](buffer),
			subscriber.WithErrorHandler[sample](logError),
		}
	}))
	if err := gw.subscribeLocal(subscribed, g); err != nil {
		return err
	}
	if err := gw.subscribeCloud(subscribed, g); err != nil {
		return err
	}

	// MaxDelay drops a sample later than the shorter period, such as those missed while the gateway was suspended.
	a := alarm.New(alarm.WithMiddleware(middleware.MaxDelay(running.RRules[0].XFreq, nil)))
	pumping := false
	err := a.Ring(ctx, idle.Between(now, end), func(ctx context.Context, at time.Time) {
		v, err := gw.device.Read(ctx)
		if err != nil {
			// The next sample is soon enough.
			log.Println(err)

			return
		}
		if (!pumping && v >= startAt) || (pumping && v < stopAt) {
			pumping = !pumping
			next := idle
			if pumping {
				next = running
			}
			// The times after at, read once fn returns, in place of what is left of the last set.
			a.Reset(next.Between(at.Add(time.Nanosecond), end))
		}
		g.Publish(ctx, sample{At: at, Velocity: v, Running: pumping})
	})

	shutdownCtx, cancel := context.WithTimeout(subscribed, shutdownTimeout)
	defer cancel()

	return errors.Join(err, g.Shutdown(shutdownCtx))
}

// subscribeLocal saves every sample, a transaction per localBatch.
func (gw gateway) subscribeLocal(ctx context.Context, g *multicast.Group[sample]) error {
	save := func(ctx context.Context, _ uint64, batch message.Batch[sample]) error {
		return gw.local.Save(ctx, slices.Collect(batch.All()))
	}
	// handle gets nothing: the holder releases batches only.
	_, err := g.Subscribe(ctx, func(ctx context.Context, _ uint64, s sample) error {
		return gw.local.Save(ctx, []sample{s})
	},
		subscriber.WithHolder(holder.NewBatch[sample](localWindow, localBatch)),
		subscriber.WithBatchHandler(save),
	)

	return err
}

// subscribeCloud sends the first sample, then each one where the pump started or stopped, retried with backoff while
// the API is down.
func (gw gateway) subscribeCloud(ctx context.Context, g *multicast.Group[sample]) error {
	// With jitter, so that gateways that failed together retry apart.
	retry := mcmiddleware.Retry[sample](mcmiddleware.RetryPolicy{
		Attempts: 5, Delay: 100 * time.Millisecond, Multiplier: 2, Jitter: 0.2,
	})
	_, err := g.Subscribe(ctx, func(ctx context.Context, _ uint64, s sample) error {
		return gw.cloud.Send(ctx, s)
	},
		subscriber.WithFilter(filter.Changed(func(prev, next sample) bool { return prev.Running != next.Running })),
		subscriber.WithMiddleware(retry),
	)

	return err
}

func logError(_ context.Context, err error) {
	log.Println(err)
}
