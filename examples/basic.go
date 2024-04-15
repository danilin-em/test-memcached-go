package main

import (
	"fmt"
	"github.com/danilin-em/test-memcached-go"
)

func main() {
	mc, err := memcached.NewMemcached("tcp", "localhost:11211")
	if err != nil {
		fmt.Println(err)
		return
	}

	original := "Hello World!\nEND\r\nBut no!\n\n"

	err = mc.Set("foo", original, 10)
	if err != nil {
		fmt.Println(err)
		return
	}

	resp, err := mc.Get("foo")
	if err != nil {
		fmt.Println(err)
		return
	}

	if original != resp {
		fmt.Println("original != resp")
		fmt.Printf("original: %q\n", original)
	}

	err = mc.Delete("foo")
	if err != nil {
		fmt.Println(err)
		return
	}

	resp, err = mc.Get("foo")
	if err != nil {
		fmt.Println(err)
		return
	}
	if resp != "" {
		fmt.Printf("resp not empty: %q\n", resp)
	}
}
