package output_formatter

import (
	"dragon-quant/model"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintRiskReport 打印风控报告
func PrintRiskReport(results []model.RiskResult) {
	fmt.Println("\n================= 老狐狸量化筛选报告 =================")
	fmt.Println("风险评分说明: 1-2分(观察) | 3分(谨慎) | 4-5分(避坑)")
	fmt.Println()

	// High Risk (Score 4-5)
	fmt.Printf("【高风险避坑区 (评分4-5)】\n")
	printTable(results, 4, 5, true)

	// Watch List (Score 1-3)
	fmt.Printf("【观察区 (评分1-3)】\n")
	printTable(results, 1, 3, false)

	// Market Assessment
	generateRiskAssessment(results)
}

func printTable(results []model.RiskResult, minScore, maxScore int, showFullReason bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if showFullReason {
		fmt.Fprintln(w, "代码\t名称\t涨幅%\tRSI\t获利盘%\t风险分\t原因")
	} else {
		fmt.Fprintln(w, "代码\t名称\t涨幅%\tRSI\t获利盘%\t5日流入\t风险分\t原因")
	}

	count := 0
	for _, r := range results {
		if r.RiskScore >= minScore && r.RiskScore <= maxScore {
			count++
			stock := r.Stock
			name := stock.Name
			if len([]rune(name)) > 4 {
				name = string([]rune(name)[:4]) // Truncate for display
			}

			if showFullReason {
				fmt.Fprintf(w, "%s\t%s\t%.1f\t%.1f\t%.1f\t%d\t%s\n",
					stock.Code, name, stock.ChangePct,
					stock.RSI6, stock.ProfitDev*100,
					r.RiskScore, r.Reason)
			} else {
				inflow5 := stock.NetInflow5Day / 10000
				fmt.Fprintf(w, "%s\t%s\t%.1f\t%.1f\t%.1f\t%.0f\t%d\t%s\n",
					stock.Code, name, stock.ChangePct,
					stock.RSI6, stock.ProfitDev*100,
					inflow5,
					r.RiskScore, r.Reason)
			}
		}
	}
	w.Flush()
	if count == 0 {
		fmt.Println("(无符合该区间的股票)")
	}
	fmt.Println()
}

func generateRiskAssessment(results []model.RiskResult) {
	fmt.Println("\n================= 市场整体风险评估 =================")

	total := len(results)
	var riskCount [6]int // 1-5, index 0 unused

	for _, r := range results {
		if r.RiskScore >= 1 && r.RiskScore <= 5 {
			riskCount[r.RiskScore]++
		}
	}

	fmt.Printf("扫描股票总数: %d\n", total)
	fmt.Printf("风险分布:\n")
	for i := 1; i <= 5; i++ {
		stars := strings.Repeat("★", i)
		fmt.Printf("  %d分(%s): %d只\n", i, stars, riskCount[i])
	}

	// Advice
	highRisk := riskCount[4] + riskCount[5]
	mediumRisk := riskCount[3]

	if float64(highRisk) > float64(total)*0.5 {
		fmt.Println("\n【老狐狸警告】: 市场过热！超过一半股票处于高风险区，建议空仓观望。")
	} else if float64(mediumRisk) > float64(total)*0.3 {
		fmt.Println("\n【老狐狸提醒】: 市场谨慎！三分一股票需谨慎对待，控制仓位，只做最强。")
	} else {
		fmt.Println("\n【老狐狸观察】: 市场情绪相对健康，仍有结构性机会，精选题材龙头。")
	}
}
