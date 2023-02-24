package main

type SomeStruct struct {
	// SomeField is a field
	SomeField  string
	SomeNumber int
}

func init() {
	println("init")
}

func main() {
	someStruct := SomeStruct{
		SomeField:  "some value",
		SomeNumber: 1,
	}
	// not count for comment
	println(getSomeField(someStruct))
}

func getSomeField(someStruct SomeStruct) string {
	result := someStruct.SomeField + " add some text"
	return result
}

func setSomeFiled(someStruct SomeStruct, someValue string) SomeStruct {
	someStruct.SomeField = someValue
	println(someStruct.SomeField)
	return someStruct
}

func someFunction() {
}
