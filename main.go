package main

import (
	"sync"
	"webcrawler/sites"
)

func main() {
	var wg = sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		sites.GetDocs()
	}()
	go func() {
		defer wg.Done()
		sites.GetNews()
	}()
	wg.Wait()
}
