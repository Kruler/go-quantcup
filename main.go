package main

import (
	"fmt"
	"time"
	"unsafe"
	// "github.com/grd/stat"
)

const (
	batchSize   int = 10
	replayCount     = 200
)

func main() {
	fmt.Println("start")
	var e Engine
	var bet int = 0
	var ordersFeed [2000000]Order
	bet = len(ordersFeed) / 2
	for i := 0; i < bet; i++ {
		ordersFeed[i].symbol = "SYM"
		ordersFeed[i].trader = fmt.Sprintf("ID%d", i)
		ordersFeed[i].side = 1
		ordersFeed[i].size = 1
		var m int = i/100 + 1
		ordersFeed[i].price = Price(m)
	}
	for i := bet; i < 2000000; i++ {
		ordersFeed[i].symbol = "SYM"
		ordersFeed[i].trader = fmt.Sprintf("ID%d", i)
		ordersFeed[i].side = 0
		ordersFeed[i].size = 1
		var m int = (i-bet)/100 + 1
		ordersFeed[i].price = Price(m)
	}
	// ordersFeed[5000000].symbol = "SYM"
	// ordersFeed[5000000].trader = "ID70001"
	// ordersFeed[5000000].side = 0
	// ordersFeed[5000000].size = 5000000
	// ordersFeed[5000000].price = 1000
	fmt.Println("man size:", unsafe.Sizeof(ordersFeed))
	e.Reset()
	begin := time.Now()
	for i := 0; i < bet; i++ {
		var order Order = ordersFeed[i]
		if order.price == 0 {
			orderID := OrderID(order.size)
			e.Cancel(orderID)
		} else {
			e.Limit(order)
		}
	}
	end := time.Now()
	begin2 := time.Now()
	for i := bet; i < len(ordersFeed); i++ {
		var order Order = ordersFeed[i]
		if order.price == 0 {
			orderID := OrderID(order.size)
			e.Cancel(orderID)
		} else {
			e.Limit(order)
		}
	}
	end2 := time.Now()
	var late int64 = end.Sub(begin).Nanoseconds()
	var lates float64 = float64(late) / 1000000000
	var mean1 float64 = float64(late) / float64(bet)
	var late2 int64 = end2.Sub(begin2).Nanoseconds()
	var lates2 float64 = float64(late2) / 1000000000
	var mean2 float64 = float64(late2) / float64(len(ordersFeed)-bet)
	fmt.Println("test order:", bet, " spend time ", lates, "s mean ", mean1)
	fmt.Println("test order:", len(ordersFeed)-bet, " spend time ", lates2, "s mean ", mean2)
	fmt.Println(e.pricePoints.Len())
	// batch latency measurements.
	// latencies := make([]time.Duration, replayCount*(len(ordersFeed)/batchSize))

	// for j := 0; j < replayCount; j++ {
	// 	e.Reset()
	// 	for i := batchSize; i < len(ordersFeed); i += batchSize {
	// 		begin := time.Now()
	// 		feed(&e, i-batchSize, i)
	// 		end := time.Now()
	// 		latencies[i/batchSize-1+(j*(len(ordersFeed)/batchSize))] = end.Sub(begin)
	// 	}
	// }

	// data := DurationSlice(latencies)

	// var mean float64 = stat.Mean(data)
	// var stdDev = stat.SdMean(data, mean)
	// var score = 0.5 * (mean + stdDev)

	// fmt.Println("test Node:", len(ordersFeed))
	// fmt.Printf("mean(latency) = %1.2f, sd(latency) = %1.2f\n", mean, stdDev)
	// fmt.Printf("You scored %1.2f. Try to minimize this.\n", score)
}

// func feed(e *Engine, begin, end int, ordersFeed [10000001]Order) {
// 	for i := begin; i < end; i++ {
// 		var order Order = ordersFeed[i]
// 		if order.price == 0 {
// 			orderID := OrderID(order.size)
// 			e.Cancel(orderID)
// 		} else {
// 			e.Limit(order)
// 		}
// 	}
// }

type DurationSlice []time.Duration

func (f DurationSlice) Get(i int) float64 { return float64(f[i]) }
func (f DurationSlice) Len() int          { return len(f) }
