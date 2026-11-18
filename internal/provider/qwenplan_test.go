package provider

import (
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// Qwen Cloud (judi0713 on Discord), Alibaba's international Qwen platform:
// pay as you go at {region}.maas.qwencloudapi.com in six regions and its
// Token Plan at token-plan.maas.qwencloudapi.com, each on keys of its own,
// as docs.qwencloud.com's "Regions and endpoints" gives them. No real Qwen
// Cloud key was tried.
func TestQwenCloud(t *testing.T) {
	p, err := FromPreset("qwencloud-token-plan")
	if err != nil {
		t.Fatal(err)
	}
	if p.Chat != "https://token-plan.maas.qwencloudapi.com/compatible-mode/v1" || p.Anthropic != "https://token-plan.maas.qwencloudapi.com/apps/anthropic" {
		t.Fatalf("plan's endpoints: %q %q", p.Chat, p.Anthropic)
	}
	if got := p.planModels(nil); len(got) != len(Preset("qwencloud-token-plan").Models) || !slices.ContainsFunc(got, func(m catalog.Model) bool { return m.ID == "qwen3.8-max" }) {
		t.Fatalf("plan's: %+v", got)
	}
	im, _ := imported("Qwen Cloud", "sk-sp-x", endpoints{chat: "https://token-plan.maas.qwencloudapi.com/compatible-mode/v1"}, nil)
	if im.Preset != "qwencloud-token-plan" {
		t.Fatalf("plan imported as %q", im.Preset)
	}

	p, err = FromPreset("qwencloud")
	if err != nil {
		t.Fatal(err)
	}
	if p.Chat != "https://ap-southeast-1.maas.qwencloudapi.com/compatible-mode/v1" || p.Anthropic != "https://ap-southeast-1.maas.qwencloudapi.com/apps/anthropic" {
		t.Fatalf("pay as you go's endpoints: %q %q", p.Chat, p.Anthropic)
	}
	pr := Preset("qwencloud")
	var ids []string
	for _, r := range pr.Regions {
		ids = append(ids, r.ID)
		if r.Chat != "https://"+r.ID+".maas.qwencloudapi.com/compatible-mode/v1" || r.Anthropic != "https://"+r.ID+".maas.qwencloudapi.com/apps/anthropic" {
			t.Errorf("region %s: %q %q", r.ID, r.Chat, r.Anthropic)
		}
	}
	if want := []string{"ap-southeast-1", "us-east-1", "eu-central-1", "ap-northeast-1", "cn-hongkong"}; !slices.Equal(ids, want) {
		t.Fatalf("regions %v, want %v", ids, want)
	}
	if pr.Regions[0].Chat != pr.Chat {
		t.Fatal("the first region isn't the default")
	}
	for _, chat := range []string{"https://eu-central-1.maas.qwencloudapi.com/compatible-mode/v1", "https://ap-southeast-1.maas.qwencloudapi.com/compatible-mode/v1"} {
		im, _ := imported("Qwen Cloud", "sk-ws-x", endpoints{chat: chat}, nil)
		if im.Preset != "qwencloud" || im.Chat != chat {
			t.Fatalf("%s imported as %q at %q", chat, im.Preset, im.Chat)
		}
	}
	q, err := ParseImport("magpie://import?preset=qwencloud&region=us-east-1")
	if err != nil || q.Chat != "https://us-east-1.maas.qwencloudapi.com/compatible-mode/v1" {
		t.Fatalf("import link: %+v (%v)", q, err)
	}
}
