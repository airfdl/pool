package pool

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestWorker(t *testing.T) {
	pool := NewWorkerPoolWithRuntimeWorkerNum(2, 2, nextNum())
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancel()
	for h := range genHandler(ctx) {
		pool.AddTaskSync(h)
	}
	time.Sleep(time.Minute)
}

func genHandler(ctx context.Context) <-chan func() {
	out := make(chan func())
	ticker := time.NewTicker(time.Millisecond * 500)
	var c int
	f := func() {
		time.Sleep(time.Second)
		fmt.Printf("GEN NUM %d\n", c)
		c++
	}
	go func() {
		for range ticker.C {
			select {
			case <-ctx.Done():
				close(out)
				return
			default:
				out <- f
			}
		}
	}()
	return out
}

func nextNum() func() int {
	var c int = 2
	rand.Seed(time.Now().Unix())
	return func() int {
		c += 1
		diff := rand.Intn(3)
		if diff%2 == 0 {
			return c - diff
		} else {
			return c + diff
		}
	}
}
