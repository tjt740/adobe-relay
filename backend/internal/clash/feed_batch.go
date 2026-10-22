package clash

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sourceConcurrency = 4

// Shared by foreground requests and monitoring, including simultaneous admins.
var sourceFetchSlots = make(chan struct{}, sourceConcurrency)

type feedJob struct {
	url   string
	index int
}
type feedResult struct {
	nodes []Node
	src   Source
}

func subscriptionURLs(raw string) ([]string, error) {
	if len(raw) > 64<<10 {
		return nil, fmt.Errorf("订阅链接文本超过 64 KiB")
	}
	urls := []string{}
	seen := map[string]bool{}
	for _, rawURL := range strings.Fields(raw) {
		if !seen[rawURL] {
			urls = append(urls, rawURL)
			seen[rawURL] = true
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("请输入订阅链接")
	}
	return urls, nil
}

// A fixed worker pool bounds both network activity and goroutines. The per-feed
// timeout starts after a shared slot is acquired, not while waiting in the queue.
func fetchFeeds(ctx context.Context, client *http.Client, jobs []feedJob, consume func(feedResult)) {
	queue := make(chan feedJob)
	results := make(chan feedResult, sourceConcurrency)
	var wg sync.WaitGroup
	for i := 0; i < min(sourceConcurrency, len(jobs)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range queue {
				select {
				case sourceFetchSlots <- struct{}{}:
				case <-ctx.Done():
					return
				}
				attempt, cancel := context.WithTimeout(ctx, 8*time.Second)
				nodes, src, _ := fetchFeed(attempt, client, job.url, job.index)
				cancel()
				<-sourceFetchSlots
				// An overall deadline says nothing about an uncompleted source.
				if ctx.Err() != nil {
					return
				}
				results <- feedResult{nodes: nodes, src: src}
			}
		}()
	}
	go func() {
		defer close(queue)
		for _, job := range jobs {
			select {
			case queue <- job:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	for result := range results {
		consume(result)
	}
}

// CheckSource downloads and parses one feed without changing saved configuration,
// node listeners, proxy bindings, or health evidence used by the scheduler.
func (m *Manager) CheckSource(ctx context.Context, raw string) (Source, error) {
	urls, err := subscriptionURLs(raw)
	if err != nil || len(urls) != 1 {
		return Source{}, fmt.Errorf("请检测单个订阅链接")
	}
	var source Source
	fetchFeeds(ctx, m.fetchClient, []feedJob{{url: urls[0], index: 1}}, func(r feedResult) { source = r.src })
	if err := ctx.Err(); err != nil {
		return Source{}, err
	}
	return source, nil
}

// Match by source and provider node name so reordering URLs cannot exchange
// proxy IDs between identically named nodes from different subscriptions.
func nodeKey(n Node) string { return n.SourceID + "\x00" + n.OriginalName }

func preserveFailedSources(previous State, nodes []Node, sources []Source) ([]Node, map[string]bool, error) {
	normalizeSources(&previous)
	existing := map[string]Node{}
	for _, n := range previous.Nodes {
		existing[nodeKey(n)] = n
	}
	failed := map[string]bool{}
	for _, src := range sources {
		if src.FetchError != "" {
			failed[src.ID] = true
		}
	}
	retained := map[string]bool{}
	for _, n := range previous.Nodes {
		if failed[n.SourceID] && n.Enabled {
			nodes = append(nodes, n)
			retained[n.Name] = true
		}
	}
	if len(nodes) > maxNodes {
		return nil, nil, fmt.Errorf("包含保留节点后超过 %d 个节点，请减少本次导入的订阅", maxNodes)
	}
	used := map[string]bool{}
	// Reserve existing names before assigning names to new nodes.
	for i, n := range nodes {
		if old, ok := existing[nodeKey(n)]; ok {
			nodes[i].Name = old.Name
			used[old.Name] = true
		}
	}
	for i, n := range nodes {
		if _, ok := existing[nodeKey(n)]; !ok {
			name := n.Name
			for suffix := 1; used[name]; suffix++ {
				tag := fmt.Sprintf(" · %s-%d", n.SourceID[:6], suffix)
				prefix := []rune(n.Name)
				prefix = prefix[:min(len(prefix), 90-len([]rune(tag)))]
				name = string(prefix) + tag
			}
			nodes[i].Name = name
			used[name] = true
		}
		// Clone before renaming: retained nodes may share the previous document.
		config := make(map[string]any, len(n.Config))
		for k, v := range n.Config {
			config[k] = v
		}
		config["name"] = nodes[i].Name
		nodes[i].Config = config
	}
	return nodes, retained, nil
}
