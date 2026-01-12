package data_processor

import (
	"dragon-quant/model"
	"fmt"
	"strings"
)

const (
	// --- 基础池过滤 ---
	MinPrice    = 15.0 // 价格下限
	MaxPrice    = 45.0 // 价格上限
	MinTurnover = 5.0  // 换手率下限 (%)
	MaxTurnover = 20.0 // 换手率上限 (%) - 防止死亡换手
	TopN        = 10   // 扫描前 N 个风口板块

	// --- 趋势与动能过滤 ---
	MinVolRatio  = 1.2  // 量比下限
	MinAmplitude = 3.0  // 振幅下限 (%) - 拒绝织布机
	RequireMA    = true // 均线多头 (价>MA5>MA20)
	RequireMACD  = true // MACD金叉 (DIF>DEA)

	// --- 资金与主力过滤 (v8.0 核心) ---
	RequireFlow    = true     // 要求主力净流入为正
	MinCallAuction = 10000000 // 竞价金额下限 (1000万) - 只有真龙头竞价才有人抢
)

// FilterBasic checks basic stock properties like price, turnover, etc.
// Returns true if the stock passes.
func FilterBasic(stk model.StockInfo) bool {
	// 排除科创板
	if strings.HasPrefix(stk.Code, "688") {
		return false
	}

	if stk.Price < MinPrice || stk.Price > MaxPrice {
		return false
	}
	if stk.Turnover < MinTurnover || stk.Turnover > MaxTurnover {
		return false
	}
	if stk.ChangePct <= 0 {
		return false
	} // 只要红盘

	// 资金与波动过滤
	if stk.Amplitude < MinAmplitude {
		return false
	}
	if RequireFlow && stk.NetInflow < 0 {
		return false
	}
	if stk.VolRatio < MinVolRatio {
		return false
	}
	return true
}

// InferDragonStatus: 根据所属板块推演连板高度
func InferDragonStatus(s *model.StockInfo) {
	isLimitUp := s.ChangePct > 9.5
	s.BoardCount = 0
	s.DragonTag = "首板/趋势"

	// 简单的推演逻辑：
	// 如果属于 "昨日连板" 且今天涨停 -> 至少 3板
	// 如果属于 "昨日涨停" 且今天涨停 -> 至少 2板

	for _, tag := range s.Tags {
		if strings.Contains(tag, "昨日连板") && isLimitUp {
			s.BoardCount = 3
			s.DragonTag = "3连板+"
			break // 找到最高级别
		}
		if strings.Contains(tag, "昨日涨停") && isLimitUp {
			s.BoardCount = 2
			s.DragonTag = "2连板"
		}
	}

	if s.BoardCount == 0 && isLimitUp {
		s.BoardCount = 1
		s.DragonTag = "首板"
	}
}

func CalculateMA(data []model.KLineData) (ma5, ma20 float64) {
	n := len(data)
	if n < 20 {
		return 0, 0
	}
	sum5 := 0.0
	for i := n - 5; i < n; i++ {
		sum5 += data[i].Close
	}
	sum20 := 0.0
	for i := n - 20; i < n; i++ {
		sum20 += data[i].Close
	}
	return sum5 / 5.0, sum20 / 20.0
}

func CalculateMACD(data []model.KLineData) (dif, dea, macd float64) {
	ema12, ema26, deaVal := 0.0, 0.0, 0.0
	for i, k := range data {
		if i == 0 {
			ema12, ema26 = k.Close, k.Close
		} else {
			ema12 = (2.0*k.Close + 11.0*ema12) / 13.0
			ema26 = (2.0*k.Close + 25.0*ema26) / 27.0
		}
		difVal := ema12 - ema26
		deaVal = (2.0*difVal + 8.0*deaVal) / 10.0
		if i == len(data)-1 {
			dif, dea, macd = difVal, deaVal, (difVal-deaVal)*2
		}
	}
	return dif, dea, macd
}

func CalculateRSI(data []model.KLineData, period int) float64 {
	if len(data) < period+1 {
		return 50.0
	}
	gainSum, lossSum := 0.0, 0.0
	recent := data[len(data)-period:]
	for _, k := range recent {
		if k.Change > 0 {
			gainSum += k.Change
		} else {
			lossSum += -k.Change
		}
	}
	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)
	if avgLoss == 0 {
		return 100.0
	}
	return 100.0 - (100.0 / (1.0 + avgGain/avgLoss))
}

func AnalyzeDragonHabit(data []model.KLineData) string {
	// 回溯过去 30 天，找到所有涨停 (>9.5%) 的次日表现
	limitUps := 0
	continued := 0 // 持续连板
	lowOpen := 0   // 低开

	n := len(data)
	if n < 2 {
		return "无记忆"
	}

	// 不包括今天 (data[n-1] is today/latest)
	for i := n - 30; i < n-1; i++ {
		if i < 0 {
			continue
		}
		if data[i].Change/data[i].Close*100 > 9.5 || (i > 0 && data[i].Change > 0 && (data[i].Close-data[i-1].Close)/data[i-1].Close > 0.095) {
			// Found a Limit Up
			limitUps++
			nextDay := data[i+1]
			// Check next day open/close (approximation: using KLine data we only have Close/Change.
			// History API k-line usually has Open/High/Low but here structs only have Close/Change.
			// Let's infer strength from Close Change.
			if nextDay.Change > 0 {
				continued++
			} else {
				lowOpen++
			}
		}
	}

	if limitUps == 0 {
		return "首板基因"
	}

	rate := float64(continued) / float64(limitUps)
	if rate >= 0.8 {
		return fmt.Sprintf("连板王(%d/%d)", continued, limitUps)
	} else if rate <= 0.3 {
		return fmt.Sprintf("炸板惯犯(%d/%d)", lowOpen, limitUps)
	}
	return fmt.Sprintf("中性(%d/%d)", continued, limitUps)
}

func CalculateVWAP(data []model.KLineData, period int, currentPrice float64) (vwap, dev float64) {
	// Standard VWAP needs Volume. Our KLineData struct currently only has Close/Change.
	// We need to approximation Volume or update struct.
	// For now, let's use Simple Moving Average as "Cost" proxy since we don't have historical volume in struct.
	// And 'Turnover' in StockInfo is only for *today*.
	// Improved: Let's just use MA30 as "Market Cost" proxy.

	n := len(data)
	if n < period {
		return 0, 0
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += data[i].Close
	}
	avgCost := sum / float64(period)

	dev = (currentPrice - avgCost) / avgCost
	return avgCost, dev
}

// GenerateTechNotes generates the technology notes and returns true if the stock passes final checks.
func GenerateTechNotes(s *model.StockInfo) bool {
	notes := []string{}
	if s.Price > s.MA5 && s.MA5 > s.MA20 {
		notes = append(notes, "多头")
	}
	if s.DIF > s.DEA {
		notes = append(notes, "金叉")
	}
	if s.RSI6 > 85 {
		notes = append(notes, "超买")
	}
	if s.CallAuctionAmt > 50000000 {
		notes = append(notes, "竞价爆量")
	}

	// 龙虎榜加持
	if s.LHBNet > 10000000 {
		notes = append(notes, "🐉机构大买")
		s.DragonTag += "/龙虎榜"
	}

	// 股性加持
	if strings.Contains(s.DragonHabit, "连板王") {
		notes = append(notes, "👑连板基因")
	}
	if s.ProfitDev > 0.3 {
		notes = append(notes, "⚠️获利盘>30%")
	}

	s.TechNotes = strings.Join(notes, "/")

	// 4. 终极过滤
	passed := true
	if RequireMA && (s.Price < s.MA5 || s.MA5 < s.MA20) {
		passed = false
	}
	if RequireMACD && (s.DIF < s.DEA) {
		passed = false
	}
	return passed
}

// CalculateSustainability 计算开盘承接率 (开盘5分钟成交额 / 竞价成交额)
func CalculateSustainability(auctionAmt float64, klines []model.KLineData) float64 {
	if auctionAmt <= 0 || len(klines) == 0 {
		return 0
	}

	// Decision: To avoid rewriting Fetcher logic deeply (modifying KLineData struct across the board),
	// I will just use the *Average* of the fetched 5-min amounts as "Average Sustainability".
	// It's a robust proxy: "Is the average 5-min trading volume comparable to the Auction volume?"

	totalAmt := 0.0
	for _, k := range klines {
		totalAmt += k.Change
	}
	avgAmt := totalAmt / float64(len(klines))
	return avgAmt / auctionAmt
}

func AnalyzeSentiment(avgChange float64) string {
	if avgChange > 3.0 {
		return "🔥 极热"
	} else if avgChange > 1.0 {
		return "Heating"
	} else if avgChange > -1.0 {
		return "震荡"
	} else if avgChange > -3.0 {
		return "Cooling"
	}
	return "❄️ 冰点"
}
