package data_processor

import (
	"dragon-quant/model"
	"strings"
)

// Analyze30mStrategy performs quantitative analysis on 30-minute K-line data.
// Returns a summary note.
func Analyze30mStrategy(klines []model.KLineData) string {
	if len(klines) < 20 {
		return "数据不足"
	}

	n := len(klines)
	current := klines[n-1]

	// 1. Calculate MA20 (30m)
	sum20 := 0.0
	for i := n - 20; i < n; i++ {
		sum20 += klines[i].Close
	}
	ma20 := sum20 / 20.0

	notes := []string{}

	// Trend Confirmation
	if current.Close > ma20 {
		notes = append(notes, "MA20趋势向上")
	} else {
		notes = append(notes, "MA20压制")
	}

	// 2. Momentum (Opening 30m of today)
	// Assuming the last 8 bars are today (4 hours / 30 mins = 8 bars).
	// This is a rough heuristic. A better way uses timestamps, but KLineData here only has Close/Change/Amount.
	// We will look at the *relative* volume of the "Dragon Head" (8th bar from end, if n >= 8)
	// But without timestamps, we can't be sure which bar is 09:30.
	// Let's assume the user runs this AFTER market close or during market.
	// If standard full day = 8 bars.

	// Better heuristic: Check recent volume spike.
	// "Dragon Head" = Huge volume on a rising bar.

	recentAvgVol := 0.0
	for i := n - 5; i < n; i++ {
		recentAvgVol += klines[i].Amount
	}
	recentAvgVol /= 5.0

	if current.Amount > recentAvgVol*2.0 && current.Change > 0 {
		notes = append(notes, "放量抢筹")
	}

	// 3. Tail Effect (Last 30m)
	// If we are at 14:30-15:00, this is the last bar.
	// Check if "Grab" or "Dump".
	// Price rose > 1% in last 30m AND Volume > Avg*1.5
	if current.Change/current.Close > 0.01 && current.Amount > recentAvgVol*1.5 {
		notes = append(notes, "尾盘抢筹")
	} else if current.Change/current.Close < -0.01 && current.Amount > recentAvgVol*1.5 {
		notes = append(notes, "尾盘出逃")
	} else if current.Close > ma20 && current.Change > 0 {
		// MA20 support + positive close
		notes = append(notes, "尾盘企稳")
	}

	// 4. Intraday Pattern (N-Shape?)
	// Simple N: Up, Down, Up.
	// Check last 3 bars.
	if n >= 3 {
		b1 := klines[n-3]
		b2 := klines[n-2]
		b3 := klines[n-1]
		if b1.Change > 0 && b2.Change < 0 && b3.Change > 0 && b3.Close > b1.Close {
			notes = append(notes, "N字反包")
		}
	}

	if len(notes) == 0 {
		return "中性"
	}
	return strings.Join(notes, "/")
}
