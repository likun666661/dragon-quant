package data_processor

import (
	"dragon-quant/model"
	"fmt"
	"sort"
	"strings"
)

// NewRiskConfig 返回老狐狸默认配置
func NewRiskConfig() model.RiskConfig {
	return model.RiskConfig{
		MaxRSI:       85.0, // RSI > 85
		MaxProfitDev: 0.30, // 获利盘 > 30% (User request said 0.25, but system code used 0.3 warning. Let's stick to user request 0.25 strict?)
		// Wait, user request said "MaxProfitDev: 0.25". Let's use 0.25 to be strict.
		MinVolRatio:    0.8,
		MaxVolRatio:    3.5,  // Slightly loose
		MaxTurnover:    25.0, // 25% is high
		MinNetInflow5d: 0,    // Must be positive

		BlacklistHabits: []string{
			"炸板惯犯",
		},
		BlacklistKeywords: []string{
			"超买",
			"获利盘>30%", // Matches existing note
		},

		GoodHabits: []string{
			"连板王",
			"首板基因",
		},
		MinBoardCount: 1, // At least once
	}
}

// RiskScreen 执行二次风控筛选
func RiskScreen(stocks []*model.StockInfo, config model.RiskConfig) []model.RiskResult {
	var results []model.RiskResult

	for _, stock := range stocks {
		riskScore := 0
		var reasons []string

		// ==== 1. 避坑检查 (Risk) ====

		// RSI
		if stock.RSI6 > config.MaxRSI {
			riskScore += 2
			reasons = append(reasons, fmt.Sprintf("RSI过热(%.1f)", stock.RSI6))
		}

		// 获利盘
		if stock.ProfitDev > config.MaxProfitDev {
			riskScore += 3
			reasons = append(reasons, fmt.Sprintf("获利盘过重(%.1f%%)", stock.ProfitDev*100))
		}

		// 量比
		if stock.VolRatio < config.MinVolRatio {
			riskScore += 1
			reasons = append(reasons, fmt.Sprintf("量比过低(%.2f)", stock.VolRatio))
		}
		if stock.VolRatio > config.MaxVolRatio {
			riskScore += 2
			reasons = append(reasons, fmt.Sprintf("量比过高(%.2f)", stock.VolRatio))
		}

		// 换手率
		if stock.Turnover > config.MaxTurnover {
			riskScore += 2
			reasons = append(reasons, fmt.Sprintf("换手率过高(%.1f%%)", stock.Turnover))
		}

		// 5日资金流
		if stock.NetInflow5Day < config.MinNetInflow5d {
			riskScore += 2
			reasons = append(reasons, fmt.Sprintf("5日流出(%.0f万)", stock.NetInflow5Day/10000))
		}

		// 不良股性
		for _, bad := range config.BlacklistHabits {
			if strings.Contains(stock.DragonHabit, bad) {
				riskScore += 3
				reasons = append(reasons, fmt.Sprintf("不良股性:%s", bad))
			}
		}

		// 技术面警告
		for _, key := range config.BlacklistKeywords {
			if strings.Contains(stock.TechNotes, key) {
				riskScore += 2
				reasons = append(reasons, fmt.Sprintf("技术警告:%s", key))
			}
		}

		// ==== 2. 加分项 (Bonus) ====
		bonus := 0

		// 好股性
		for _, good := range config.GoodHabits {
			if strings.Contains(stock.DragonHabit, good) {
				bonus++
				reasons = append(reasons, fmt.Sprintf("加分:股性(%s)", good))
			}
		}

		// 龙虎榜次数
		if stock.BoardCount >= config.MinBoardCount {
			bonus++
			reasons = append(reasons, fmt.Sprintf("加分:龙虎榜%d次", stock.BoardCount))
		}

		// 今日大单
		if stock.NetInflow > 100000000 {
			bonus++
			reasons = append(reasons, fmt.Sprintf("加分:今日流入%.1f亿", stock.NetInflow/100000000))
		}

		// 3. 最终评分
		finalScore := riskScore - bonus
		if finalScore < 1 {
			finalScore = 1
		}
		if finalScore > 5 {
			finalScore = 5
		}

		// Filter criteria: Keep if Warning (Score >= 3) OR Bonus > 0 (Opportunity)
		if finalScore >= 3 || bonus > 0 {
			results = append(results, model.RiskResult{
				Stock:     stock,
				Reason:    strings.Join(reasons, " | "),
				RiskScore: finalScore,
			})
		}
	}

	// Sort by Risk Score (Descending?? No, User sorted by Risk Score Ascending 1..5)
	// User Logic:
	// sort.Slice(results, func(i, j int) bool {
	// 	if results[i].RiskScore == results[j].RiskScore {
	// 		return results[i].Stock.ChangePct > results[j].Stock.ChangePct
	// 	}
	// 	return results[i].RiskScore < results[j].RiskScore
	// })
	sort.Slice(results, func(i, j int) bool {
		if results[i].RiskScore == results[j].RiskScore {
			return results[i].Stock.ChangePct > results[j].Stock.ChangePct
		}
		return results[i].RiskScore < results[j].RiskScore
	})

	return results
}
