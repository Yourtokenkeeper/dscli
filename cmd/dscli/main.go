// Command dscli talks to the DeepSeek consumer backend (chat.deepseek.com)
// with the credentials the official Android app already holds.
//
// It is deliberately small and dependency-light: one binary, no runtime, no
// API key. Usage:
//
//	dscli whoami
//	dscli list [-n 10]
//	dscli history <session-id> [-n 20]
//	dscli send "问题" [-session id] [-think] [-search] [-quiet]
//	dscli tail <session-id> [-interval 3]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Yourtokenkeeper/dscli/internal/api"
	"github.com/Yourtokenkeeper/dscli/internal/config"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "whoami":
		err = cmdWhoami(ctx)
	case "list":
		err = cmdList(ctx, os.Args[2:])
	case "history":
		err = cmdHistory(ctx, os.Args[2:])
	case "send":
		err = cmdSend(ctx, os.Args[2:])
	case "tail":
		err = cmdTail(ctx, os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("dscli %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dscli — 在终端里使用 DeepSeek C 端（复用 App 登录态）

  whoami                     查看当前登录身份
  list [-n 10]               最近会话
  history <session> [-n 20]  某会话的消息
  send "文本" [-session id] [-think] [-search] [-quiet]
  tail <session> [-interval 3]  轮询该会话，新消息实时打印
  version

凭证来源：-session-file / $DS_TOKEN / $DS_SESSION_FILE /
~/.dsh/deepseek/session.json（{"token":"...","uid":"..."}）
`)
}

// client builds an API client from flags/env/defaults.
func client(sessionFile string) (*api.Client, error) {
	s, err := config.Load(sessionFile)
	if err != nil {
		return nil, err
	}
	return api.New(s.Token, s.DeviceID), nil
}

func cmdWhoami(ctx context.Context) error {
	c, err := client("")
	if err != nil {
		return err
	}
	u, err := c.Whoami(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("id:     %s\nemail:  %s\nmobile: %s\nstatus: %d\n", u.ID, u.Email, u.Mobile, u.Status)
	return nil
}

func cmdList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	n := fs.Int("n", 10, "number of sessions")
	sessionFile := fs.String("session-file", "", "path to session json")
	fs.Parse(args)

	c, err := client(*sessionFile)
	if err != nil {
		return err
	}
	list, err := c.Sessions(ctx, *n)
	if err != nil {
		return err
	}
	for _, s := range list {
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Printf("%s  %s\n", s.ID, title)
	}
	return nil
}

func cmdHistory(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	n := fs.Int("n", 20, "number of messages")
	sessionFile := fs.String("session-file", "", "path to session json")
	fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("history needs a session id")
	}
	c, err := client(*sessionFile)
	if err != nil {
		return err
	}
	msgs, err := c.History(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if len(msgs) > *n {
		msgs = msgs[len(msgs)-*n:]
	}
	for _, m := range msgs {
		who := "DS"
		if strings.HasPrefix(strings.ToUpper(m.Role), "U") {
			who = "我"
		}
		text := m.Text()
		if len([]rune(text)) > 500 {
			text = string([]rune(text)[:500]) + "…"
		}
		fmt.Printf("[%s] %s\n", who, text)
	}
	return nil
}

func cmdSend(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	session := fs.String("session", "", "existing session id")
	sessionFile := fs.String("session-file", "", "path to session json")
	think := fs.Bool("think", false, "enable deep thinking")
	search := fs.Bool("search", false, "enable web search")
	quiet := fs.Bool("quiet", false, "print only the final text")
	fs.Parse(args)

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		b, _ := os.ReadFile("/dev/stdin")
		prompt = strings.TrimSpace(string(b))
	}
	if prompt == "" {
		return fmt.Errorf("send needs a prompt")
	}
	c, err := client(*sessionFile)
	if err != nil {
		return err
	}
	sid, err := c.Ask(ctx, prompt, api.AskOptions{
		SessionID: *session,
		Thinking:  *think,
		Search:    *search,
		OnText: func(s string) {
			if !*quiet {
				fmt.Print(s)
			}
		},
	})
	if err != nil {
		return err
	}
	if !*quiet {
		fmt.Printf("\n\n# session=%s\n", sid)
	}
	return nil
}

func cmdTail(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	interval := fs.Int("interval", 3, "poll interval in seconds")
	sessionFile := fs.String("session-file", "", "path to session json")
	fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("tail needs a session id")
	}
	sid := fs.Arg(0)
	c, err := client(*sessionFile)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	if msgs, err := c.History(ctx, sid); err == nil {
		for _, m := range msgs {
			seen[m.MessageID] = true
		}
	}
	fmt.Fprintf(os.Stderr, "# watching %s (Ctrl-C to stop)\n", sid)
	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			msgs, err := c.History(ctx, sid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "# poll failed: %v\n", err)
				continue
			}
			for _, m := range msgs {
				if seen[m.MessageID] {
					continue
				}
				seen[m.MessageID] = true
				who := "DS"
				if strings.HasPrefix(strings.ToUpper(m.Role), "U") {
					who = "我"
				}
				ts := time.Unix(int64(m.InsertedAt), 0).Format("15:04:05")
				fmt.Printf("\n[%s] %s: %s\n", ts, who, m.Text())
			}
		}
	}
}
