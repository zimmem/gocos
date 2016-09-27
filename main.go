package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"

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
	pushRemote = push.Arg("remote", "cos path").Required().Strings()

	rm          = app.Command("rm", "rm files or directories from cos")
	rmPath      = rm.Arg("remote", "remote cos path").Required().String()
	rmRecursive = rm.Flag("recursive", "remove directories and their contents recursively").Short('r').Bool()

	retry       = app.Command("retry", "run retry script")
	retryScript = retry.Arg("script", "retry script path").ExistingFile()
)

func loadConfig(configFile *string) {

	var file *os.File
	var err error
	if len(*configFile) > 0 {
		_, err = os.Stat(*configFile)
		exitIfErr(err)
	} else {
		*configFile = "cos.config.json"
		_, err = os.Stat(*configFile)
		if err != nil && os.IsNotExist(err) {
			var usr, _ = user.Current()
			*configFile = usr.HomeDir + string(os.PathSeparator) + "cos.config.json"
			_, err = os.Stat(*configFile)
			if err != nil && os.IsNotExist(err) {
				fmt.Printf("config not exist")
				os.Exit(1)
			}
		}
	}
	fmt.Printf("load config from  : %s\n", *configFile)
	file, _ = os.Open(*configFile)
	text, e := ioutil.ReadAll(file)
	exitIfErr(e)
	println(text)
}

func exitIfErr(e error) {
	if e != nil {
		log.Fatal(e)
		os.Exit(1)
	}
}

func main() {

	var command = kingpin.MustParse(app.Parse(os.Args[1:]))
	loadConfig(configFile)
	switch command {

	case pull.FullCommand():
		loadConfig(configFile)
		println(*pullRemote)

	case push.FullCommand():
		println(*pushLocal)
	}

}
