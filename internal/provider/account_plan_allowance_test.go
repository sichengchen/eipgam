package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// aliAllowanceServer is Alibaba Cloud's Singapore console answering an
// access key's Token Plan as the Usage page's card reads it: the token
// minted for the access key, then the plan's windows, counting the reads.
func aliAllowanceServer(t *testing.T) *atomic.Int32 {
	t.Helper()
	asked := &atomic.Int32{}
	five := time.Now().Add(3 * time.Hour).UnixMilli()
	week := time.Now().Add(4 * 24 * time.Hour).UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Host") {
		case "modelstudio.ap-southeast-1.aliyuncs.com":
			io.WriteString(w, `{"cliAccessToken":"tok-allowance"}`)
		case "bailian-singapore-cs.alibabacloud.com":
			if r.Header.Get("Authorization") != "Bearer tok-allowance" {
				t.Errorf("read with %q", r.Header.Get("Authorization"))
			}
			asked.Add(1)
			io.WriteString(w, `{"code":"200","data":{"success":true,"DataV2":{"data":{"data":{`+
				`"per5HourPercentage":0.008,"per5HourResetTime":`+unixText(five)+`,`+
				`"per1WeekPercentage":0.8714,"per1WeekResetTime":`+unixText(week)+`}}}}}`)
		default:
			t.Errorf("asked %s%s", r.Header.Get("X-Host"), r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = rewrite{srv}
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	// no console token kept from another test
	aliTokensMu.Lock()
	aliTokens = map[string]aliToken{}
	aliTokensMu.Unlock()
	t.Cleanup(func() {
		aliTokensMu.Lock()
		aliTokens = map[string]aliToken{}
		aliTokensMu.Unlock()
	})
	return asked
}

var aliPlan = Provider{ID: "alibaba-token-plan", Name: "Alibaba Cloud Token Plan", Key: "sk-sp-k",
	Chat: "https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1", AccessKeyID: "AK1", SecretAccessKey: "SK1"}

// chen on Discord: the Usage page had a plan's windows read with the
// account's access key while the Routing page said "one key" of it, as
// routing never read them. A key whose account tells its plan's windows
// (Alibaba's, Tencent's, StepFun's) is weighed by them as a Kimi Code or
// GLM Coding Plan key is.
func TestAccountPlanKeyAllowance(t *testing.T) {
	keyLimitsHome(t)
	forgetPlanQuotas()
	t.Cleanup(forgetPlanQuotas)
	asked := aliAllowanceServer(t)
	if err := Save(aliPlan); err != nil {
		t.Fatal(err)
	}
	if KeyAllowanceID(aliPlan) == "" {
		t.Fatal("an Alibaba Token Plan key has no windows routing reads")
	}
	a := waitAllowance(t, aliPlan, func(Allowance) bool { return true })
	used, renews := a.For("qwen3.6-plus", time.Now())
	if used < 87 || used > 87.2 || len(renews) != 2 {
		t.Fatalf("used %v, renews %v of %+v", used, renews, a)
	}
	waitRead(t)
	if n := asked.Load(); n != 1 {
		t.Fatalf("asked %d times", n)
	}

	// what the Usage page's card reads is what routing goes by at once
	keyAllowances.Lock()
	keyAllowances.m = nil
	keyAllowances.Unlock()
	lastQuotas.Lock()
	lastQuotas.m, lastQuotas.loaded = nil, false
	lastQuotas.Unlock()
	if qs := PlanQuotas(context.Background()); len(qs) != 1 || len(qs[0].Windows) != 2 {
		t.Fatalf("cards: %+v", qs)
	}
	if a, ok := KeyAllowance(aliPlan); !ok || len(a) != 2 {
		t.Fatalf("after the card's read: %v %+v", ok, a)
	}
	if n := asked.Load(); n != 2 {
		t.Fatalf("asked %d times: routing asked again what the card had just read", n)
	}

	// restarted: the card kept on disk stands in until a read is back
	keyAllowances.Lock()
	keyAllowances.m = nil
	keyAllowances.Unlock()
	lastQuotas.Lock()
	lastQuotas.m, lastQuotas.loaded = nil, false
	lastQuotas.Unlock()
	if a, ok := KeyAllowance(aliPlan); !ok || len(a) != 2 {
		t.Fatalf("from the card on disk: %v %+v", ok, a)
	}
	waitRead(t)

	// pay-as-you-go, or a plan without the account's access key, has none
	// and is never asked
	for _, p := range []Provider{
		{ID: "qwen-payg", Key: "sk-k", Chat: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", AccessKeyID: "AK1", SecretAccessKey: "SK1"},
		{ID: "ali-nosecret", Key: "sk-sp-k", Chat: "https://token-plan.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1", AccessKeyID: "AK1"},
		{ID: "tc-payg", Key: "tc-k", Chat: "https://tokenhub-intl.tencentmaas.com/v1", AccessKeyID: "AK1", SecretAccessKey: "SK1"},
	} {
		if _, ok := KeyAllowance(p); ok || KeyAllowanceID(p) != "" {
			t.Errorf("%s reads key windows", p.ID)
		}
	}
	// a Tencent Token Plan Enterprise key and a StepFun key are read too
	for _, p := range []Provider{
		{ID: "tc-plan", Key: "tc-k", Chat: "https://tokenhub-intl.tencentmaas.com/plan/v3", AccessKeyID: "AK1", SecretAccessKey: "SK1"},
		{ID: "stepfun", Key: "sf-k", Chat: "https://api.stepfun.ai/step_plan/v1"},
	} {
		if KeyAllowanceID(p) == "" {
			t.Errorf("%s reads no key windows", p.ID)
		}
	}
	waitRead(t)
}

// A StepFun key is weighed by its Step Plan's windows, read with
// magpie's sign-in; one not signed in is no windows, not an error.
func TestStepPlanKeyAllowance(t *testing.T) {
	keyLimitsHome(t)
	forgetPlanQuotas()
	t.Cleanup(forgetPlanQuotas)
	f := newFakeStepFun(t, "ai")
	soon, later := unixText(time.Now().Add(2*time.Hour).Unix()), unixText(time.Now().Add(5*24*time.Hour).Unix())
	f.rateBody = `{"five_hour_usage_left_rate":0.6,"five_hour_usage_reset_time":"` + soon + `","weekly_usage_left_rate":"0.9","weekly_usage_reset_time":"` + later + `"}`
	p := Provider{ID: "stepfun", Name: "StepFun", Key: "sf-k", Chat: "https://api.stepfun.ai/step_plan/v1"}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	KeyAllowance(p)
	waitRead(t)
	if a, ok := KeyAllowance(p); ok {
		t.Fatalf("known without a sign-in: %+v", a)
	}

	acc, ref := stepfunTokens("4161", time.Now().Add(time.Hour), true)
	f.access.Store(acc)
	if err := SaveStepFunSession(context.Background(), "stepfun", "ai", acc, ref, "web-1"); err != nil {
		t.Fatal(err)
	}
	forgetKeyAllowances()
	a := waitAllowance(t, p, func(Allowance) bool { return true })
	// 40% of the five hours used, the most of the three
	if used, _ := a.For("step-3.5-flash", time.Now()); used < 39.9 || used > 40.1 {
		t.Fatalf("used %v of %+v", used, a)
	}
	waitRead(t)
}

func unixText(n int64) string { return strconv.FormatInt(n, 10) }
