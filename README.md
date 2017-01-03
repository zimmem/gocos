# gocos

## config

### 配置文件查找顺序

gocos 按以下顺序查找配置文件， 一但找到就不继续找

1. `--config` 参数指定配置文件路径
2. 当前目录下 `cos.config.json`
3. $HOME 目录下 `.cos.config.json`

### 配置文件格式

```
{
    "AppID": "<your APPID>",
    "SecretID": "<your SecretID>",
    "SecretKey": "<your SecretKey>",
    "Bucket": "<your Bucket>",
    "Local": "gz",
    "UseHttps" : false
}

```

## usage

```
$gocos
usage: gocos [<flags>] <command> [<args> ...]

A command-line tool for qcloud cos.

Flags:
  --help           Show context-sensitive help (also try --help-long and
                   --help-man).
  --config=CONFIG  config file path

Commands:
  help [<command>...]
    Show help.

  env
    show current config

  ls <path>
    list file at directories

  stat [<flags>] <path>
    statFile

  pull <remote> [<local>]
    pull from cos to local

  push [<flags>] <local> <remote>
    pusl local file to cos

  rm [<flags>] <remote>
    rm files or directories from cos

  mv [<flags>] <src> <target>
    mv file from src to target.

  cat <remote>
    cat file from cos.
```
