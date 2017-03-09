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
	"io"
	"bytes"
)

var Failure = false

type Command interface {
	Execute(cosClient *cosclient.CosClient)
	Name() string
}

type ListCommand struct {
	clause *kingpin.CmdClause
	remote *string
}

func (l *ListCommand) Name() string {
	return l.clause.FullCommand()
}
func (l *ListCommand) Execute(cosClient *cosclient.CosClient) {
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

func (s *StatCommand) Execute(cosClient *cosclient.CosClient) {
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

func (s *StatCommand) Name() string {
	return s.clause.FullCommand()
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

func (l *PullCommand) Name() string {
	return l.clause.FullCommand()
}

func (p *PullCommand) Execute(cosClient *cosclient.CosClient) {
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
			cosClient.Download(remote, local, 0, nil)
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
	cover  *bool
}

func (l *PushCommand) Name() string {
	return l.clause.FullCommand()
}

func (p *PushCommand) Execute(cosClient *cosclient.CosClient) {
	cosClient.Upload(*p.local, *p.remote, *p.cover)
}

func CreatePushCommand(app *kingpin.Application) *PushCommand {
	clause := app.Command("push", "pusl local file to cos")
	return &PushCommand{
		clause:clause,
		local: clause.Arg("local", "local path").Required().ExistingFileOrDir(),
		remote:  clause.Arg("remote", "remote path").Required().String(),
		cover: clause.Flag("force", "force cover files on cos").Short('f').Bool(),
	}
}

type RmCommand struct {
	clause    *kingpin.CmdClause
	remote    *string
	recursive *bool
	force     *bool
}

func (l *RmCommand) Name() string {
	return l.clause.FullCommand()
}

func (r *RmCommand) Execute(cosClient *cosclient.CosClient) {
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

type MvCommand struct {
	clause *kingpin.CmdClause
	src    *string
	target *string
	force  *bool
}

func (l *MvCommand) Name() string {
	return l.clause.FullCommand()
}

func (r *MvCommand) Execute(cosClient *cosclient.CosClient) {
	cosClient.Move(*r.src, *r.target, *r.force)
}

func CreateMvCommand(app *kingpin.Application) *MvCommand {
	clause := app.Command("mv", "mv file from src to target.")

	return &MvCommand{
		clause:clause,
		src:   clause.Arg("src", "source file ").Required().String(),
		target : clause.Arg("target", "target location or filename").Required().String(),
		force:clause.Flag("force", "force  cover target file").Short('f').Bool(),
	}
}

type CatCommand struct {
	clause *kingpin.CmdClause
	remote *string
}

func (l *CatCommand) Name() string {
	return l.clause.FullCommand()
}

func (r *CatCommand) Execute(cosClient *cosclient.CosClient) {
	callback := func(reader  io.Reader) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		fmt.Println(buf.String())
	}
	cosClient.DownloadStream(*r.remote, callback);
}

func CreateCatCommand(app *kingpin.Application) *CatCommand {
	clause := app.Command("cat", "cat file from cos.")

	return &CatCommand{
		clause:clause,
		remote:   clause.Arg("remote", "cos file ").Required().String(),
	}
}

type UpdateCommand struct {
	clause    *kingpin.CmdClause
	remote    *string
	authority *string
}

func (l *UpdateCommand) Name() string {
	return l.clause.FullCommand()
}

func (r *UpdateCommand) Execute(cosClient *cosclient.CosClient) {
	response := cosClient.UpdateAuthority(r.remote, r.authority);
	if response.Code == 0 {
		fmt.Printf("success")
	}else{
		fmt.Fprintf(os.Stderr, "error: %d - %s\n",response.Code, response.Message)
	}
}

func CreateUpdateCommand(app *kingpin.Application) *UpdateCommand {
	clause := app.Command("update", "update file authority.")

	return &UpdateCommand{
		clause:clause,
		remote:   clause.Arg("remote", "cos file ").Required().String(),
		authority : clause.Flag("authority", "authority for file : eInvalid / eWRPrivate / eWPrivateRPublic").Short('a').Required().String(),
	}
}