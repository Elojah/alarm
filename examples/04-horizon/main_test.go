package main

func Example() {
	main()

	// Output:
	// read 10ms
	// read 20ms
	// read 200ms
	// fired 10ms
	// fired 20ms
	// read 210ms
	// fired 200ms
	// fired 210ms
}
