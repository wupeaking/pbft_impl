package main

import (
	"math/rand"
	"time"
)

func main() {
	t := time.NewTimer(5 * time.Second)
	t2 := time.NewTimer(5 * time.Second)

	go func() {
		for {
			time.Sleep(time.Duration(rand.Intn(4)) * time.Second)
			if !t2.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t2.Reset(5 * time.Second)
			println(time.Now().Unix(), " 重置...")
		}
	}()

	for {
		select {
		case <-t.C:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(5 * time.Second)
			println(time.Now().Unix(), " 超时....")
		case <-t2.C:
			println(time.Now().Unix(), " 超时2....")

		}
	}
}
