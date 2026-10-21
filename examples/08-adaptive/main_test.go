package main

func Example() {
	main()

	// Output:
	// local: 0s, 0.2 mm/s, idle
	// local: 100ms, 0.3 mm/s, idle
	// local: 200ms, 3.8 mm/s, running
	// local: 210ms, 4.1 mm/s, running
	// local: 220ms, 4.5 mm/s, running
	// local: 230ms, 4.4 mm/s, running
	// local: 240ms, 4.6 mm/s, running
	// local: 250ms, 4.3 mm/s, running
	// local: 260ms, 4.5 mm/s, running
	// local: 270ms, 4.4 mm/s, running
	// local: 280ms, 2.1 mm/s, running
	// local: 290ms, 1.2 mm/s, running
	// local: 300ms, 0.6 mm/s, idle
	// local: 400ms, 0.3 mm/s, idle
	// local: 500ms, 0.2 mm/s, idle
	// cloud: 0.2 mm/s, idle at 0s
	// cloud: 3.8 mm/s, running at 200ms
	// cloud: 0.6 mm/s, idle at 300ms
}
