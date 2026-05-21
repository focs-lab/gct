package main

// import (
// 	"testing"
//  	"context"
// 	"time"
//  "github.com/stretchr/testify/assert"
// )

// func TestSelect_1(t *testing.T) {
// 	ch1 := make(chan int, 2)
// 	ch2 := make(chan int, 3)
// 	ch3 := make(chan int, 1)

// 	select {
// 	case ch1 <- 1:
// 		println("case 1")
// 	case x := <-ch2:
// 		_ = x
// 		println("case 2")
// 	case <-ch3:
// 		println("case 3")
// 	default:
// 		println("default case")
// 	}
// }

// func TestSelect_2(t *testing.T) {
// 	ch := make(chan int, 1)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		}
// 	}()

// 	select {
// 	case <-ch:
// 		println("received")
// 	}
// }

// func TestSelect_3(t *testing.T) {
// 	ch := make(chan int)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		}

// 	}()

// 	select {
// 	case <-ch:
// 		println("received")
// 	}

// }

// func TestSelect_4(t *testing.T) {
// 	ch1 := make(chan int)
// 	ch2 := make(chan int, 1)

// 	go func() {
// 	select {
// 	case ch1 <- 1:
// 		println("sent ch1")
// 	case <- ch2:
// 		println("received ch2")
// 	}

// 	}()

// 	select {
// 	case <-ch1:
// 		println("received ch1")
// 	case ch2 <- 1:
// 		println("sent ch2")
// 	}
// }

// func TestSelect_5(t *testing.T) {
// 	ch := make(chan int)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		}
// 	}()

// 	close(ch)
// }

// func TestSelect_6(t *testing.T) {
// 	ch := make(chan int)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		}

// 	}()

// 	go func() {
// 		select {
// 		case <-ch:
// 			println("recv")
// 		}

// 	}()
// 	close(ch)
// }

// func TestSelect_7(t *testing.T) {
// 	ch1 := make(chan int)
// 	ch2 := make(chan int)

// 	// donec = ch2
// 	// stopc = ch1

// 	go func() {
// 		select {
// 		case ch1 <- 1:
// 			println("case 1")
// 		case <-ch2:
// 			println("case 2")
// 		}
// 		<-ch2
// 	}()

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond)
// 		defer func() {
// 			ticker.Stop()
// 			close(ch2)
// 		}()

// 		for {
// 			println("entered this for loop")

// 			select {
// 			case <-ticker.C:
// 				println("tokenTicker.C")
// 			case <-ch1:
// 				println("ch1")
// 				return
// 			}
// 		}
// 	}()
// }

// func TestSelect_8(t *testing.T) {
// 	ch1 := make(chan int)
// 	ch2 := make(chan int, 1)

// 	go func() {
// 	select {
// 	case <- ch2:
// 		println("received ch2")
// 		select {
// 		case ch1 <- 1:
// 			println("sent ch1")
// 		}
// 	}

// 	}()

// 	select {
// 	case ch2 <- 1:
// 		println("sent ch2")
// 		select{
// 		case <-ch1:
// 			println("received ch1")
// 		}
// 	}
// }

// func TestSelect_9(t *testing.T) {
// 	ch := make(chan int)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		}
// 	}()

// 	select {
// 	case <-ch:
// 		println("received")
// 	default:
// 		println("default")
// 	}
// }

// func TestSelect_10(t *testing.T) {
// 	ch := make(chan int)

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		default:
// 		    println("default goroutine created")
// 		}
// 	}()

// 	select {
// 	case <-ch:
// 		println("received")
// 	default:
// 		println("default")
// 	}
// }

// func TestSelect_11(t *testing.T) {
// 	ch := make(chan int)
// 	ctx := context.WithoutCancel(context.Background())

// 	go func() {
// 		select {
// 		case ch <- 1:
// 			println("sent")
// 		default:
// 		    println("default goroutine created")
// 		}
// 	}()

// 	select {
// 	case <-ctx.Done():
// 		println("received")
// 	case x := <- ch:
// 		println(x)
// 	default:
// 		println("default")
// 	}
// }

// func TestSelect_12(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	ch1 <- 1
// 	ch2 <- 2

// 	var x int

// 	select {
// 	case x = <-ch1:
// 		println("received ch1")
// 	case x = <-ch2:
// 		println("received ch2")
// 	}
// 	assert.Equal(t, x, 1)
// }

// func TestSelect_13(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	select {
// 	default:
// 	case <-ch1:
// 		println("received ch1")
// 	case <-ch2:
// 		println("received ch2")
// 	}
// }

// func TestSelect_14(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	select {
// 	case <-ch1:
// 		println("received ch1")
// 	default:
// 	case <- ch1:
// 		println("received ch2")
// 	case <-ch2:
// 		println("received ch2")
// 	}
// }

// func TestSelect_15(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	for i := 0; i < 2; i++ {
// 		select {
// 		case <-ch1:
// 			println("received ch1")
// 		case <-ch1:
// 			println("received ch2")
// 		case <-ch2:
// 			println("received ch2")
// 		default:
// 		}
// 	}
// }

// func TestSelect_16(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	for i := 0; i < 2; i++ {
// 		select {
// 		case <-ch1:
// 			return
// 		case <-ch2:
// 			return
// 		default:
// 			return
// 		}
// 	}
// }

// func TestSelect_17(t *testing.T) {
// 	ch1 := make(chan int, 1)
// 	ch2 := make(chan int, 1)

// 	for i := 0; i < 2; i++ {
// 		select {
// 		case <-ch1:
// 			return
// 		case <-ch2:
// 			return
// 		}
// 	}
// }
