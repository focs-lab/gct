package test_programs

// import (
// 	"fmt"
// 	"sync"
// 	"time"
//     "testing"
// )

// func TestWaitGroup_1(t *testing.T) {
//     var wg sync.WaitGroup
//     wg.Add(1)
//     go func() {
//         wg.Done()
//     }()
//     wg.Wait()
// }

// func TestWaitGroup_2(t *testing.T) {
//     var wg sync.WaitGroup
//     wg.Add(3)

//     for i := 0; i < 3; i++ {
//         go func() {
//             wg.Done()
//         }()
//     }

//     wg.Wait()
// }

// func TestWaitGroup_3(t *testing.T) {
//     wg := &sync.WaitGroup{}
//     wg.Add(1)
//     wg.Done()
//     wg.Wait()
// }

// func TestWaitGroup_4(t *testing.T) {
//     var wg sync.WaitGroup
//     wg.Add(1)
//     go func() {
//         defer wg.Done()
//     }()
//     wg.Wait()
// }

// func TestWaitGroup_5(t *testing.T) {
// 	var wg sync.WaitGroup

// 	wg.Go(func() {
// 		fmt.Println("Task 1 started")
// 		time.Sleep(100 * time.Millisecond)
// 		fmt.Println("Task 1 done")
// 	})

// 	wg.Go(func() {
// 		fmt.Println("Task 2 started")
// 		time.Sleep(50 * time.Millisecond)
// 		fmt.Println("Task 2 done")
// 	})

// 	wg.Wait()
// }
