package main

func Example() {
	main()

	// Output:
	// local: saved 10 readings, 0s to 100ms
	// local: saved 10 readings, 110ms to 200ms
	// local: saved 9 readings, 210ms to 290ms
	// cloud: unavailable
	// cloud: sent 20.0°C, 20.6°C, 21.1°C, 22.4°C, 23.0°C, 23.5°C, 24.2°C, 24.7°C, 25.3°C
}
