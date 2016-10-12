package cmd

import (
	"gocos/cosclient"
	"gopkg.in/alecthomas/kingpin.v2"
	"encoding/json"
	"text/template"
	"os"
	"fmt"
	"strings"
	"sync"
)

type Command interface {
	Execute(cosClient *cosclient.CosClient)
	Name() string
}

type ListCommand struct {
	clause *kingpin.CmdClause
	remote *string
}

func (l ListCommand) Name() string {
	return l.clause.FullCommand()
}
func (l ListCommand) Execute(cosClient *cosclient.CosClient) {
	cosClient.List(*l.remote, "")
}

func CreateListCommand(app *kingpin.Application) *ListCommand {
	clause := app.Command("ls", "list file at directories")
	return &ListCommand{
		clause:clause,
		remote:  clause.Arg("path", "path on cos").Required().String(),
	}
}

type StatCommand struct {
	clause *kingpin.CmdClause
	remote *string
	format *string
}

func (s StatCommand) Execute(cosClient *cosclient.CosClient) {
	stat := cosClient.StatFile(*s.remote)
	if *s.format == "" {
		r, _ := json.MarshalIndent(stat, "", "  ")
		fmt.Println(string(r))
	} else {
		t, err := template.New("StatFormat").Parse(*s.format);
		if err != nil {
			panic(err)
		}
		t.Execute(os.Stdout, stat)
	}
}

func (l StatCommand) Name() string {
	return l.clause.FullCommand()
}

func CreateStatCommand(app *kingpin.Application) *StatCommand {
	clause := app.Command("stat", "statFile")
	return &StatCommand{
		clause : clause,
		remote: clause.Arg("path", "path on cos").Required().String(),
		format: clause.Flag("format", "format by golang template").Short('f').String(),
	}
}

type PullCommand struct {
	clause *kingpin.CmdClause
	remote *string
	local  *string
}

func (l PullCommand) Name() string {
	return l.clause.FullCommand()
}

func (p PullCommand) Execute(cosClient *cosclient.CosClient) {
	local := *p.local
	if local == "" {
		local, _ = os.Getwd()
		if !strings.HasSuffix(local, string(os.PathSeparator)) {
			local += string(os.PathSeparator)
		}
	}

	if strings.HasSuffix(*p.remote, "/") && !strings.HasSuffix(local, string(os.PathSeparator)) {
		local += string(os.PathSeparator)
	}

	threadPoll := make(chan int, 20)
	for i := 0; i < 20; i++ {
		threadPoll <- 1
	}
	waitter := &sync.WaitGroup{}
	pull(cosClient, *p.remote, local, threadPoll, waitter)
	waitter.Wait()
}

func pull(cosClient *cosclient.CosClient, remote, local string, threads chan int, waitter  *sync.WaitGroup) {

	if strings.HasSuffix(remote, "/") {
		os.MkdirAll(local, 0766)

		goon := true
		context := ""
		for ; goon; {
			resp := cosClient.ExecList(remote, context)
			context = resp.Data.Context
			goon = context != ""
			for _, v := range resp.Data.Infos {
				tremote := remote + v.Name
				tlocal := local + strings.Replace(v.Name, "/", string(os.PathSeparator), -1)
				pull(cosClient, tremote, tlocal, threads, waitter)
			}
		}

	} else {
		if strings.HasSuffix(local, string(os.PathSeparator)) {
			fname := remote[strings.LastIndex(remote, "/"):]
			local += fname
		}
		waitter.Add(1)
		<-threads
		go func(cosClient *cosclient.CosClient, remote, local string) {
			defer func() {
				threads <- 1
				waitter.Done()
			}()
			cosClient.Download(remote, local)
		}(cosClient, remote, local)

	}
}

func CreatePullCommand(app *kingpin.Application) *PullCommand {
	clause := app.Command("pull", "pull from cos to local")
	return &PullCommand{
		clause:clause,
		remote:  clause.Arg("remote", "remote path").Required().String(),
		local: clause.Arg("local", "local path").String(),
	}
}

type PushCommand struct {
	clause *kingpin.CmdClause
	local  *string
	remote *string
}

func (l PushCommand) Name() string {
	return l.clause.FullCommand()
}

func (p PushCommand) Execute(cosClient *cosclient.CosClient) {
	cosClient.Upload(*p.local, *p.remote, false)
}

func CreatePushCommand(app *kingpin.Application) *PushCommand {
	clause := app.Command("push", "pusl local file to cos")
	return &PushCommand{
		clause:clause,
		local: clause.Arg("local", "local path").Required().String(),
		remote:  clause.Arg("remote", "remote path").Required().String(),
	}
}

type RmCommand struct {
	clause    *kingpin.CmdClause
	remote    *string
	recursive *bool
	force     *bool
}

func (l RmCommand) Name() string {
	return l.clause.FullCommand()
}

func (r RmCommand) Execute(cosClient *cosclient.CosClient) {
	cosClient.DeleteResource(*r.remote, *r.recursive, *r.force)
}

func CreateRmCommand(app *kingpin.Application) *RmCommand {
	clause := app.Command("rm", "rm files or directories from cos")

	return &RmCommand{
		clause:clause,
		remote:   clause.Arg("remote", "remote cos path").Required().String(),
		recursive : clause.Flag("recursive", "remove directories and their contents recursively").Short('r').Bool(),
		force:clause.Flag("force", "force rm even has children").Short('f').Bool(),
	}
}