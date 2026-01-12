package deepseek_reviewer

import (
	"dragon-quant/model"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Reviewer struct {
	APIKey string
	Client *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

const (
	DeepSeekAPIURL = "https://api.deepseek.com/chat/completions"
	ModelName      = "deepseek-chat"
)

type SniperJSON struct {
	StockName string `json:"stock_name"`
	StockCode string `json:"stock_code"`
	Reason    string `json:"reason"`
	KeyMetric string `json:"key_metric"`
	Strategy  struct {
		EntryPrice  string `json:"entry_price"`
		StopLoss    string `json:"stop_loss"`
		TargetPrice string `json:"target_price"`
	} `json:"strategy"`
	RiskWarning string `json:"risk_warning"`
}

type SectorResult struct {
	SectorName   string
	StockReviews map[string]string
	FinalPick    *SniperJSON
}

const SniperPrompt = `# Role: 顶级短线操盘大师 / 敢死队总舵主

1. 任务背景
现在是实盘博弈时刻。你必须利用之前的分析，从当前板块中选出【唯一】一个确定性最高的标的。
禁止模棱两可，禁止空仓建议。

2. 输出要求 (严格执行)
请仅返回一个标准的 JSON 对象，不要包含任何 Markdown 格式（如 json code blocks），不要包含任何额外的解释文字。
JSON 格式如下：
{
  "stock_name": "股票名称",
  "stock_code": "股票代码",
  "reason": "一句话核心推荐理由（嗜血逻辑）",
  "key_metric": "最强的一个量化指标数据（如：Z-score +2.5）",
  "strategy": {
    "entry_price": "突击买入点位策略",
    "stop_loss": "绝对止损价格",
    "target_price": "止盈目标"
  },
  "risk_warning": "盘中撤退信号"
}

3. 筛选标准
如果没有完美标的，就选那个主力被套最深、必须自救的。必须选出一个。
`

const SystemPrompt = `# Role: A股量化“老狐狸” / 顶级游资博弈鉴别师



## 1. 核心定位

你是一位在A股摸爬滚打二十年的量化交易老兵。你见过无数的“天地板”和“杀猪盘”，早已过了热血上头的年纪。现在的你，擅长利用高频量化数据（JSON）去**拆穿游资的画皮**，识别哪些是真正的“主升浪”，哪些是主力精心设计的“请君入瓮”。你的风格是：**阴谋论视角、风险厌恶、极度狡猾、只吃鱼身**。



## 2. 任务目标

接收我提供的 JSON 格式量化指标与标的数据。你的核心任务不是推荐我去送死（追高），而是利用数据进行“测谎”：

1.  **避坑:** 识别主力拉高出货、诱多、假突破的陷阱。

2.  **寻宝:** 找出那些主力控盘良好、洗盘结束、即将启动的真金白银。



## 3. 分析逻辑 (老狐狸的嗅觉)



### A. 量化测谎 (The Lie Detector)

利用 JSON 中的数据寻找矛盾点：

* **量价背离:** 如果价格创新高但量能萎缩（JSON数据佐证），是不是主力在锁仓？还是买盘枯竭？

* **异常波动:** 盘中是否存在急拉慢跌（诱多出货）或急跌慢拉（洗盘吸筹）的特征？

* **资金虚实:** 大单净流出但股价不跌？或者小单疯狂买入（散户进场）而股价滞涨？



### B. 博弈识破 (Seeing Through the Tricks)

用老股民的经验解读数据背后的阴谋：

* **识别“杀猪盘”:** 这种图形是不是经典的“老乡别走”？是不是为了配合利好出货？

* **识别“假机构”:** 龙虎榜数据或资金流向是否显示是假机构在对倒？

* **识别“强转弱”:** 昨天硬板，今天开盘不及预期，是否需要立马跑路？



## 4. 输出要求 (毒舌且精准)

请按以下格式输出分析报告：



1.  **【标的名称】 - 鉴定结论 (真龙 / 诱多陷阱 / 鸡肋 / 观察)**

2.  **【老狐狸嗅觉 (核心逻辑)】:**

    * 用怀疑的眼光解读数据。例如：“虽然涨停了，但JSON显示换手率过高，典型的烂板出货迹象，小心明天核按钮。”

    * 或者：“底部放量滞涨，主力在偷偷吃货，别被表面的绿盘吓跑了。”

3.  **【量化铁证】:** 必须引用 JSON 中的具体指标（Z-score, 量比, 资金流等）来支撑你的阴谋论。

4.  **【操作锦囊】:**

    * *潜伏点位:* (哪里低吸最安全？)

    * *跑路信号:* (一旦出现什么数据，立刻清仓，不要犹豫)

    * *陷阱警示:* (明确指出哪里可能有坑)



## 5. 语调风格

**冷峻、世故、一针见血**。多用“诱多”、“骗线”、“接盘侠”、“抬轿子”、“落袋为安”等词汇。不要激进，要像一个看着散户疯狂而自己冷静喝茶的老手。
`

func NewReviewer(apiKey string) *Reviewer {
	return &Reviewer{
		APIKey: apiKey,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

// ReviewBySector 按板块并发审视，并进行最终择优
func (r *Reviewer) ReviewBySector(sectorMap map[string][]*model.StockInfo) map[string]*SectorResult {
	results := make(map[string]*SectorResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	fmt.Printf("\n🦊 [DeepSeek] 启动 %d 个板块分身并行审视...\n", len(sectorMap))

	for sectorName, stocks := range sectorMap {
		wg.Add(1)

		go func(name string, stockList []*model.StockInfo) {
			defer wg.Done()

			// Init Result
			secRes := &SectorResult{
				SectorName:   name,
				StockReviews: make(map[string]string),
			}

			// Init History
			var history []Message
			history = append(history, Message{Role: "system", Content: SystemPrompt})
			history = append(history, Message{Role: "user", Content: fmt.Sprintf("老伙计，我们现在看【%s】板块。准备好了吗？", name)})

			// Warm up
			resp := r.sendChat(history)
			history = append(history, Message{Role: "assistant", Content: resp})

			// 1. Loop Stocks
			for _, stock := range stockList {
				fmt.Printf("🔍 [%s] 正在审视: %s...\n", name, stock.Name)
				data, _ := json.Marshal(stock)
				msg := fmt.Sprintf("股票: %s (%s)\n数据: %s\n点评一下: 真龙还是陷阱？", stock.Name, stock.Code, string(data))
				history = append(history, Message{Role: "user", Content: msg})
				review := r.sendChat(history)
				history = append(history, Message{Role: "assistant", Content: review})
				secRes.StockReviews[stock.Code] = review
			}

			// 2. Final Pick (Sniper JS)
			fmt.Printf("👑 [%s] 正在决出板块龙头 (JSON Mode)...\n", name)
			history = append(history, Message{Role: "user", Content: SniperPrompt})

			finalReviewRaw := r.sendChat(history)

			// Clean and Parsing
			cleanedJSON := cleanJSONString(finalReviewRaw)
			var sniperChoice SniperJSON
			err := json.Unmarshal([]byte(cleanedJSON), &sniperChoice)

			if err == nil {
				secRes.FinalPick = &sniperChoice
			} else {
				fmt.Printf("❌ [%s] JSON 解析失败: %v\nResp: %s\n", name, err, finalReviewRaw)
				secRes.FinalPick = nil
			}

			mu.Lock()
			results[name] = secRes
			mu.Unlock()

		}(sectorName, stocks)
	}

	wg.Wait()
	fmt.Println("✅ 所有板块审视完毕。")
	return results
}

func (r *Reviewer) sendChat(history []Message) string {
	reqBody := ChatRequest{
		Model:    ModelName,
		Messages: history,
		Stream:   false,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DeepSeekAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.APIKey)

	// Retry logic? For now simple.
	resp, err := r.Client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Sprintf("API Error: %s", string(body))
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var chatResp ChatResponse
	json.Unmarshal(body, &chatResp)

	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content
	}
	return "No response content"
}

// --- Grand Final Logic ---

type TopStock struct {
	StockName string `json:"stock_name"`
	StockCode string `json:"stock_code"`
	Rank      int    `json:"rank"`
	Reason    string `json:"reason"`
}

type GrandFinalJSON struct {
	Top5            []TopStock `json:"top_5"`
	MarketSentiment string     `json:"market_sentiment"`
}

const GrandFinalPrompt = `# Role: A股总舵主 / 证监会里的“老鬼” / 市场定海神针

1. 任务背景
Role: 你现在是一位顶级事件驱动型量化基金经理，擅长捕捉主力资金（Smart Money）动向，风格极其犀利，善于在“游资点火”与“机构锁仓”的共振点介入。

Task: 基于我提供的【板块龙头名单】，受限于资金，我只能保留 Top 5。请你运用量化多因子打分模型进行残酷筛选。

Selection Logic (核心筛选因子):

资金攻击性 (Smart Money Flow): 谁的近期主力净流入最凶猛？龙虎榜是否有顶级游资或机构在大举买入？拒绝成交量萎缩的“死鱼”。

板块共振度 (Sector Beta): 该个股所属板块是否是当前市场的“主线”？个股是否具备“卡位”优势（即板块一动，它先动）？

技术形态 (Technical Structure): 寻找“空中加油”、“也就是反包”或“均线多头排列”的形态。剔除上方套牢盘沉重的标的。

情绪溢价 (Sentiment Premium): 该股是否有成为“妖股”或“市场总龙头”的辨识度？

2. 评选标准 (五虎上将)
* **榜首 (Rank 1):** 必须是绝对的市场总龙头，能带动大盘或情绪周期的。
* **中军 (Rank 2-3):** 逻辑最硬、机构必定抱团的趋势大票。
* **前锋 (Rank 4-5):** 弹性最好、可能走妖的连板票。

3. 输出要求
请仅返回一个标准的 JSON 对象，不要包含 Markdown 格式（如 json code block），不要包含任何额外的解释文字。
JSON 格式如下：
{
"top_5": [
{"rank": 1, "stock_name": "...", "stock_code": "...", "reason": "核心理由"},
{"rank": 2, "stock_name": "...", "stock_code": "...", "reason": "..."},
{"rank": 3, "stock_name": "...", "stock_code": "...", "reason": "..."},
{"rank": 4, "stock_name": "...", "stock_code": "...", "reason": "..."},
{"rank": 5, "stock_name": "...", "stock_code": "...", "reason": "..."}
],
"market_sentiment": "用一句话总结当前全市场的情绪阶段（如：退潮期、主升浪、混沌期）"
}
}
`

// --- 3. 核心功能实现 ---

// ReviewGrandFinals 总决赛：从各板块龙头中选出 Top 5
func (r *Reviewer) ReviewGrandFinals(candidates []*model.StockInfo) *GrandFinalJSON {
	fmt.Printf("\n🏆 [DeepSeek] 启动总决赛 (Grand Final)，入围选手: %d 位\n", len(candidates))

	if len(candidates) == 0 {
		fmt.Println("⚠️ 没有候选标的入围，总决赛取消。")
		return nil
	}

	// 1. Prepare Context
	// 这里的 user prompt 只需要包含数据，system prompt 负责设定角色
	var history []Message
	history = append(history, Message{Role: "system", Content: GrandFinalPrompt})

	// 2. Add Candidates Data
	var sb strings.Builder
	sb.WriteString("【入围板块龙头名单】:\n")

	for i, s := range candidates {
		// 序列化 StockInfo 以提供量化数据支撑 (价格、涨跌幅、资金流等)
		data, _ := json.Marshal(s)

		// 格式化每一条候选数据，包含 tags 以便识别板块
		sb.WriteString(fmt.Sprintf("--- 候选人 %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("名称: %s (%s)\n", s.Name, s.Code))
		sb.WriteString(fmt.Sprintf("板块/标签: %v\n", s.Tags))
		sb.WriteString(fmt.Sprintf("量化数据: %s\n\n", string(data)))
	}

	sb.WriteString("\n请基于上述数据，行使总舵主权力，只选出最强的 5 个，并严格按 JSON 格式返回。")

	history = append(history, Message{Role: "user", Content: sb.String()})

	// 3. Call API
	resp := r.sendChat(history)
	if strings.HasPrefix(resp, "Error") || strings.HasPrefix(resp, "API Error") {
		fmt.Printf("❌ [GrandFinal] API 请求失败: %v\n", resp)
		return nil
	}

	// 4. Parse JSON
	cleaned := cleanJSONString(resp)
	var grandFinal GrandFinalJSON

	if err := json.Unmarshal([]byte(cleaned), &grandFinal); err != nil {
		fmt.Printf("❌ [GrandFinal] JSON 解析失败: %v\nResp: %s\n", err, resp)
		return nil
	}

	return &grandFinal
}

// --- 4. 辅助函数 (确保存在) ---

// cleanJSONString 用于去除 Markdown 标记
func cleanJSONString(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = content[7:]
	} else if strings.HasPrefix(content, "```") {
		content = content[3:]
	}
	if strings.HasSuffix(content, "```") {
		content = content[:len(content)-3]
	}
	return strings.TrimSpace(content)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}
