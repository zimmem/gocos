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
	"gocos/cmd"
)

var (
	app = kingpin.New("gocos", "A command-line tool for qcloud cos.")
	configFile = app.Flag("config", "config file path").String()

	env = app.Command("env", "show current config")
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
			*configFile = getUserHome() + string(os.PathSeparator) + ".cos.config.json"
		}
	}
	//fmt.Printf("load config from  : %s\n", *configFile)
	file, _ = os.Open(*configFile)
	defer file.Close()
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

func getUserHome() string {
	h := os.Getenv("HOME")
	if len(h) == 0 {
		var usr, _ = user.Current()
		h = usr.HomeDir
	}
	return h
}

func main() {
	//os.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")

	defer func() {
		e := recover()
		if e != nil {
			fmt.Fprintf(os.Stderr, "%+v", e)
			os.Exit(-1)
		}
	}()

	commands := []cmd.Command{
		cmd.CreateListCommand(app),
		cmd.CreateStatCommand(app),
		cmd.CreatePullCommand(app),
		cmd.CreatePushCommand(app),
		cmd.CreateRmCommand(app),
		cmd.CreateMvCommand(app),
		cmd.CreateCatCommand(app),
	}

	var command = kingpin.MustParse(app.Parse(os.Args[1:]))

	client := &cosclient.CosClient{}
	json.Unmarshal(loadConfig(configFile), client)

	if env.FullCommand() == command {
		fmt.Println("config: " + config)

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(client)
	} else {
		for _, comm := range commands {
			if comm.Name() == command {
				comm.Execute(client)
			}
		}
	}

}
