package main

import (
	"encoding/json"
	"fmt"
	"gocos/cosclient"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app        = kingpin.New("gocos", "A command-line toll for qcloud cos.")
	configFile = app.Flag("config", "config file path").String()

	pull       = app.Command("pull", "pull from cos to local")
	pullRemote = pull.Arg("remote", "remote path").Required().String()
	pullLoacl  = pull.Arg("local", "local path").Required().String()

	push       = app.Command("push", "push from local to cos")
	pushLocal  = push.Arg("local", "local path").Required().String()
	pushRemote = push.Arg("remote", "cos path").Required().String()

	rm          = app.Command("rm", "rm files or directories from cos")
	rmPath      = rm.Arg("remote", "remote cos path").Required().String()
	rmRecursive = rm.Flag("recursive", "remove directories and their contents recursively").Short('r').Bool()
	rmForce     = rm.Flag("force", "force rm even has children").Short('f').Bool()

	ls     = app.Command("ls", "list file at directories")
	lsPath = ls.Arg("path", "path on cos").Required().String()

	stat     = app.Command("stat", "statFile")
	statPath = stat.Arg("path", "path on cos").Required().String()

	// retry       = app.Command("retry", "run retry script")
	// retryScript = retry.Arg("script", "retry script path").ExistingFile()

	env    = app.Command("env", "show current config")
	config = ""
)

func loadConfig(configFile *string) []byte {

	var file *os.File
	var err error
	if len(*configFile) > 0 {
		_, err = os.Stat(*configFile)
		exitIfErr(err)
	} else {
		*configFile = "cos.config.json"
		_, err = os.Stat(*configFile)
		if err != nil && os.IsNotExist(err) {
			*configFile = getUserHOme() + string(os.PathSeparator) + "cos.config.json"
		}
	}
	//fmt.Printf("load config from  : %s\n", *configFile)
	file, _ = os.Open(*configFile)
	text, e := ioutil.ReadAll(file)
	exitIfErr(e)
	config, _ = filepath.Abs(*configFile)
	return text
}

func exitIfErr(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "%s\n", e)
		os.Exit(1)
	}
}

func getUserHOme() string {
	h := os.Getenv("HOME")
	if len(h) == 0 {
		var usr, _ = user.Current()
		h = usr.HomeDir
	}
	return h
}

func main() {
	os.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")

	defer func() {
		e := recover()
		if e != nil {
			fmt.Fprintf(os.Stderr, "%+v", e)
		}
	}()

	var command = kingpin.MustParse(app.Parse(os.Args[1:]))

	client := &cosclient.CosClient{}
	json.Unmarshal(loadConfig(configFile), client)

	switch command {

	case pull.FullCommand():
		client.Download(*pullRemote, *pullLoacl)

	case push.FullCommand():
		client.Upload(*pushLocal, *pushRemote, false)

	case rm.FullCommand():
		client.DeleteResource(*rmPath, *rmRecursive, *rmForce)

	case ls.FullCommand():
		client.List(*lsPath, "")
	case stat.FullCommand():
		client.StatFile(*statPath)

	case env.FullCommand():
		fmt.Println("config: " + config)

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(client)
	}

}
