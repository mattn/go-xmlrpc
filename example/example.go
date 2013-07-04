package main

import (
	"github.com/mattn/go-xmlrpc"
	"fmt"
	"log"
)

func main() {
	res, e := xmlrpc.Call(
		"http://your-blog.example.com/xmlrpc.php",
		"metaWeblog.getRecentPosts",
		"blog-id",
		"user-id",
		"password",
		10)
	if e != nil {
		log.Fatal(e)
	}
	for _, p := range res.(xmlrpc.Array) {
		for k, v := range p.(xmlrpc.Struct) {
			fmt.Printf("%s=%v\n", k, v)
		}
		fmt.Println()
	}
}
