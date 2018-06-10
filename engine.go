/*****************************************************************************
 *                QuantCup 1:   Price-Time Matching Engine
 *
 * Submitted by: voyager (Go port by rdingwall@gmail.com)
 *
 * Design Overview:
 *   In this implementation, the limit order book is represented using
 *   a flat linear array (pricePoints), indexed by the numeric price value.
 *   Each entry in this array corresponds to a specific price point and holds
 *   an instance of struct pricePoint. This data structure maintains a list
 *   of outstanding buy/sell orders at the respective price. Each outstanding
 *   limit order is represented by an instance of struct orderBookEntry.
 *
 *   askMin and bidMax are global variables that maintain starting points,
 *   at which the matching algorithm initiates its search.
 *   askMin holds the lowest price that contains at least one outstanding
 *   sell order. Analogously, bidMax represents the maximum price point that
 *   contains at least one outstanding buy order.
 *
 *   When a Buy order arrives, we search the book for outstanding Sell orders
 *   that cross with the incoming order. We start the search at askMin and
 *   proceed upwards, incrementing askMin until:
 *     a) The incoming Buy order is filled.
 *     b) We reach a price point that no longer crosses with the incoming
 *        limit price (askMin > BuyOrder.price)
 *     In case b), we create a new orderBookEntry to record the
 *     remainder of the incoming Buy order and add it to the global order
 *     book by appending it to the list at pricePoints[BuyOrder.price].
 *
 *  Incoming Sell orders are handled analogously, except that we start at
 *  bidMax and proceed downwards.
 *
 *  Although this matching algorithm runs in linear time and may, in
 *  degenerate cases, require scanning a large number of array slots,
 *  it appears to work reasonably well in practice, at least on the
 *  simulated data feed (score_feed.h). The vast majority of incoming
 *  limit orders can be handled by examining no more than two distinct
 *  price points and no order requires examining more than five price points.
 *
 *  To avoid incurring the costs of dynamic heap-based memory allocation,
 *  this implementation maintains the full set of orderBookEntry instances
 *  in a statically-allocated contiguous memory arena (arenaBookEntries).
 *  Allocating a new entry is simply a matter of bumping up the orderID
 *  counter (curOrderID) and returning a pointer to arenaBookEntries[curOrderID].
 *
 *  To cancel an order, we simply set its size to zero. Notably, we avoid
 *  unhooking its orderBookEntry from the list of active orders in order to
 *  avoid incurring the costs of pointer manipulation and conditional branches.
 *  This allows us to handle order cancellation requests very efficiently; the
 *  current implementation requires only one memory store instruction on
 *  x86_64. During order matching, when we walk the list of outstanding orders,
 *  we simply skip these zero-sized entries.
 *
 *  The current implementation uses a custom version of strcpy() to copy the string
 *  fields ("symbol" and "trader") between data structures. This custom version
 *  has been optimized for the case STRINGLEN=5 and implements loop unrolling
 *  to eliminate the use of induction variables and conditional branching.
 *
 *  The memory layout of struct orderBookEntry has been optimized for
 *  efficient cache access.
 *****************************************************************************/

package main

import (
	"fmt"
	"github.com/yasushi-saito/rbtree"
)

type MyItem struct {
	key   Price
	value pricePoint
}

type Engine struct {

	// Optional callback function that is called when a trade is executed.
	Execute func(Execution)

	// An array of pricePoint structures representing the entire limit order
	// book.
	// pricePoints [uint(maxPrice) + 1]pricePoint
	pricePoints *rbtree.Tree
	curOrderID  OrderID // Monotonically-increasing orderID.
	askMin      uint    // Minimum Ask price.
	bidMax      uint    // Maximum Bid price.

	// Statically-allocated memory arena for order book entries. This data
	// structure allows us to avoid the overhead of heap-based memory
	// allocation.
	bookEntries [maxNumOrders]orderBookEntry
}

// struct orderBookEntry: Describes a single outstanding limit order (Buy or
// Sell).
type orderBookEntry struct {
	size   Size
	next   *orderBookEntry
	trader string
}

// struct pricePoint: Describes a single price point in the limit order book.
type pricePoint struct {
	listHead *orderBookEntry
	listTail *orderBookEntry
}

const maxNumOrders uint = 15000010

func (e *Engine) Reset() {
	// for _, pricePoint := range e.pricePoints {
	// 	pricePoint.listHead = nil
	// 	pricePoint.listTail = nil
	// }

	for _, bookEntry := range e.bookEntries {
		bookEntry.size = 0
		bookEntry.next = nil
		bookEntry.trader = ""
	}

	e.Execute = msss
	e.pricePoints = rbtree.NewTree(func(a, b rbtree.Item) int { return int(a.(MyItem).key - b.(MyItem).key) })
	// tree := rbtree.NewTree(func(a, b rbtree.Item) int { return a.(MyItem).key - b.(MyItem).key })
	// tree.Insert(MyItem{10, "value10"})
	// tree.Insert(MyItem{12, "value12"})
	// fmt.Println("Get(10) ->", tree.Get(MyItem{10, ""}).(MyItem).value)
	// fmt.Println("Get(11) ->", tree.Get(MyItem{11, ""}))

	e.curOrderID = 0
	e.askMin = uint(maxPrice) + 1
	e.bidMax = uint(minPrice) - 1
	e.pricePoints.Insert(MyItem{key: Price(e.askMin)})
	e.pricePoints.Insert(MyItem{key: Price(e.bidMax)})
}

// Process an incoming limit order.
func (e *Engine) Limit(order Order) OrderID {
	var price Price = order.price
	var orderSize Size = order.size
	var tmp MyItem
	if order.side == Bid { // Buy order.
		// Look for outstanding sell orders that cross with the incoming order.
		if uint(price) >= e.askMin {
			// ppEntry := &e.pricePoints[e.askMin]

			if e.pricePoints.Get(MyItem{key: Price(e.askMin)}) == nil {
				e.pricePoints.Insert(MyItem{key: Price(e.askMin)})
			}

			tmp.key = Price(e.askMin)
			pentry := e.pricePoints.Get(tmp).(MyItem).value
			ppEntry := &pentry
			for {
				bookEntry := ppEntry.listHead

				for bookEntry != nil {
					if bookEntry.size < orderSize {
						execute(e.Execute, order.symbol, order.trader, bookEntry.trader, Price(e.askMin), bookEntry.size)

						orderSize -= bookEntry.size
						bookEntry = bookEntry.next
					} else {
						execute(e.Execute, order.symbol, order.trader, bookEntry.trader, Price(e.askMin), orderSize)

						if bookEntry.size > orderSize {
							bookEntry.size -= orderSize
						} else {
							bookEntry = bookEntry.next
						}

						ppEntry.listHead = bookEntry
						e.pricePoints.Insert(MyItem{key: Price(e.askMin), value: pentry})
						e.curOrderID++

						return e.curOrderID
					}
				}

				// We have exhausted all orders at the askMin price point. Move
				// on to the next price level.
				ppEntry.listHead = nil
				tmp.key = Price(e.askMin)
				nodeg := e.pricePoints.FindGE(tmp).Next()
				if nodeg.Limit() == true {
					break
				}
				e.askMin = uint(nodeg.Item().(MyItem).key)
				tmp.key = Price(e.askMin)
				tmpp := e.pricePoints.Get(tmp).(MyItem).value
				ppEntry = &tmpp

				// if uint(price) < e.askMin {
				// 	break
				// }
			}
		}

		e.curOrderID++
		entry := &e.bookEntries[e.curOrderID]
		entry.size = orderSize
		entry.trader = order.trader
		tmp.key = price
		var priceentry pricePoint
		if e.pricePoints.Get(MyItem{key: price}) != nil {
			priceentry = e.pricePoints.Get(MyItem{key: price}).(rbtree.Item).(MyItem).value
		}
		ppInsertOrder(&priceentry, entry)
		e.pricePoints.Insert(MyItem{key: price, value: priceentry})

		if e.bidMax < uint(price) {
			e.bidMax = uint(price)
		}

		return e.curOrderID
	} else { // Sell order.
		// Look for outstanding Buy orders that cross with the incoming order.
		if uint(price) <= e.bidMax {
			if e.pricePoints.Get(MyItem{key: Price(e.bidMax)}) == nil {
				e.pricePoints.Insert(MyItem{key: Price(e.bidMax)})
			}

			tmp.key = Price(e.bidMax)
			pentry := e.pricePoints.Get(tmp).(MyItem).value
			ppEntry := &pentry

			for {
				bookEntry := ppEntry.listHead

				for bookEntry != nil {
					if bookEntry.size < orderSize {
						execute(e.Execute, order.symbol, bookEntry.trader, order.trader, Price(e.bidMax), bookEntry.size)

						orderSize -= bookEntry.size
						bookEntry = bookEntry.next
					} else {
						execute(e.Execute, order.symbol, bookEntry.trader, order.trader, Price(e.bidMax), orderSize)

						if bookEntry.size > orderSize {
							bookEntry.size -= orderSize
						} else {
							bookEntry = bookEntry.next
						}
						ppEntry.listHead = bookEntry
						e.pricePoints.Insert(MyItem{key: Price(e.bidMax), value: pentry})
						e.curOrderID++
						return e.curOrderID
					}
				}

				// We have exhausted all orders at the bidMax price point. Move
				// on to the next price level.
				ppEntry.listHead = nil
				tmp.key = Price(e.bidMax)
				nodel := e.pricePoints.FindLE(tmp).Prev()
				if nodel.Limit() == true {
					break
				}
				e.bidMax = uint(nodel.Item().(MyItem).key)
				tmp.key = Price(e.bidMax)
				tmpp := e.pricePoints.Get(tmp).(MyItem).value
				ppEntry = &tmpp

				// if uint(price) > e.bidMax {
				// 	break
				// }
			}
		}

		e.curOrderID++
		entry := &e.bookEntries[e.curOrderID]
		entry.size = orderSize
		entry.trader = order.trader
		tmp.key = price
		var priceentry pricePoint
		if e.pricePoints.Get(MyItem{key: price}) != nil {
			priceentry = e.pricePoints.Get(MyItem{key: price}).(rbtree.Item).(MyItem).value
		}
		ppInsertOrder(&priceentry, entry)
		e.pricePoints.Insert(MyItem{key: price, value: priceentry})

		if e.askMin > uint(price) {
			e.askMin = uint(price)
		}
		return e.curOrderID
	}
}

func (e *Engine) Cancel(orderID OrderID) {
	e.bookEntries[orderID].size = 0
}

// Report trade execution.
func execute(hook func(Execution), symbol, buyTrader, sellTrader string, price Price, size Size) {
	if hook == nil {
		fmt.Println("debug")
		return // No callback defined.
	}

	if size == 0 {
		fmt.Println("debug2")
		return // Skip orders that have been cancelled.
	}

	var exec Execution = Execution{symbol: symbol, price: price, size: size}

	exec.side = Bid
	exec.trader = buyTrader

	hook(exec) // Report the buy-side trade.

	exec.side = Ask
	exec.trader = sellTrader
	hook(exec) // Report the sell-side trade.
}

// Insert a new order book entry at the tail of the price point list.
func ppInsertOrder(ppEntry *pricePoint, entry *orderBookEntry) {
	if ppEntry.listHead != nil {
		ppEntry.listTail.next = entry
	} else {
		ppEntry.listHead = entry
	}
	ppEntry.listTail = entry
}

func msss(exec Execution) {
	if exec.price == 0 {
		fmt.Println("symbol ", exec.symbol, "trader ", exec.trader, "price ", exec.price, "size ", exec.size)
	}
}
