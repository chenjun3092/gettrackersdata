//批量获取搜狐股票成交明细数据。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var startDate *string = flag.String("d", "Null", "please input a startDate like 20131104")
var num *int = flag.Int("n", 0, "please input a num like 1024")
var stockType *string = flag.String("s", "sh", "please input a stockType like sh")
var dataType *string = flag.String("t", "chddata", "please input a dataType like chddata")

const (
	UA = "Golang Downloader from Ijibu.com"
)

func main() {
	flag.Usage = show_usage
	flag.Parse()
	var (
		stockCodeFile string
		stockPre      string
		logFileDir    string
		downDir       string
		downFileExt   string
		getUrl        string
	)

	if *startDate == "Null" || *num == 0 {
		show_usage()
		return
	}

	urlsConfig := map[string]string{}
	//成交明细
	urlsConfig["cjmx"] = "http://quotes.money.163.com/cjmx/2013/?/??.xls"
	//每日收盘情况
	urlsConfig["chddata"] = "http://quotes.money.163.com/service/chddata.html?code=??&start=?&end=?&fields=TCLOSE;HIGH;LOW;TOPEN;LCLOSE;CHG;PCHG;TURNOVER;VOTURNOVER;VATURNOVER;TCAP;MCAP"
	//历史资金流向
	urlsConfig["lszjlx"] = "http://quotes.money.163.com/trade/lszjlx_?,?.html"

	cupNum := runtime.NumCPU()
	runtime.GOMAXPROCS(cupNum) //设置cpu的核的数量，从而实现高并发
	c := make(chan int, *num)
	if *stockType == "sh" {
		stockCodeFile = "./ini/shang_new.ini"
		stockPre = "0"
	} else {
		stockCodeFile = "./ini/shen_new.ini"
		stockPre = "1"
	}

	//日志文件目录，文件下载地址，下载后保存的文件类型
	if *dataType == "cjmx" {
		logFileDir = "./log/163/cjmx/" + *stockType + "/"
		downDir = "./data/163/cjmx/" + *stockType + "/" + *startDate + "/"
		downFileExt = ".xls"
	} else if *dataType == "chddata" {
		logFileDir = "./log/163/chddata/" + *stockType + "/"
		downDir = "./data/163/chddata/" + *stockType + "/" + *startDate + "/"
		downFileExt = ".csv"
	} else if *dataType == "lszjlx" {
		logFileDir = "./log/163/lszjlx/" + *stockType + "/"
		downDir = "./data/163/lszjlx/" + *stockType + "/" + *startDate + "/"
		downFileExt = ".html"
	}
	if !isDirExists(logFileDir) { //目录不存在，则进行创建。
		err := os.MkdirAll(logFileDir, 777) //递归创建目录，linux下面还要考虑目录的权限设置。
		if err != nil {
			panic(err)
		}
	}
	if !isDirExists(downDir) { //目录不存在，则进行创建。
		err := os.MkdirAll(downDir, 777) //递归创建目录，linux下面还要考虑目录的权限设置。
		if err != nil {
			panic(err)
		}
	}

	logfile, _ := os.OpenFile(logFileDir+*startDate+".log", os.O_RDWR|os.O_CREATE, 0)
	logger := log.New(logfile, "\r\n", log.Ldate|log.Ltime|log.Llongfile)

	fh, ferr := os.Open(stockCodeFile)
	if ferr != nil {
		return
	}
	defer fh.Close()
	inputread := bufio.NewReader(fh)

	for i := 1; i <= *num; i++ { //加入goroutine缓冲，4个执行完了再执行下面的4个
		input, _ := inputread.ReadString('\n')
		code := strings.TrimSpace(input)

		if *dataType == "cjmx" {
			getUrl = "http://quotes.money.163.com/cjmx/2013/" + *startDate + "/" + stockPre + code + ".xls"
		} else if *dataType == "chddata" {
			getUrl = "http://quotes.money.163.com/service/chddata.html?code=" + stockPre + code + "&start=" + *startDate + "&end=" + *startDate + "&fields=TCLOSE;HIGH;LOW;TOPEN;LCLOSE;CHG;PCHG;TURNOVER;VOTURNOVER;VATURNOVER;TCAP;MCAP"
		} else if *dataType == "lszjlx" {
			getUrl = "http://quotes.money.163.com/trade/lszjlx_" + code + ",0.html"
		}

		go func(logger *log.Logger, logfile *os.File, code string, downDir string, getUrl string, downFileExt string) {
			getShangTickerTables(logger, logfile, code, downDir, getUrl, downFileExt)
			c <- 0
		}(logger, logfile, code, downDir, getUrl, downFileExt)

		if i%4 == 0 { //并发默认为4
			time.Sleep(4 * time.Second) //加入执行缓冲，否则同时发起大量的tcp连接，操作系统会直接返回错误。
		}

	}
	defer logfile.Close()
	for j := 0; j < *num; j++ {
		<-c
	}
}

func getShangTickerTables(logger *log.Logger, logfile *os.File, code string, downDir string, getUrl string, downFileExt string) {
	fileName := downDir + code + downFileExt
	f, err := os.OpenFile(fileName, os.O_CREATE, 0666) //其实这里的 O_RDWR应该是 O_RDWR|O_CREATE，也就是文件不存在的情况下就建一个空文件，但是因为windows下还有BUG，如果使用这个O_CREATE，就会直接清空文件，所以这里就不用了这个标志，你自己事先建立好文件。
	if err != nil {
		panic(err)
	}

	defer f.Close()

	var req http.Request
	req.Method = "GET"
	req.Close = true
	req.URL, err = url.Parse(getUrl)
	if err != nil {
		panic(err)
	}

	header := http.Header{}
	header.Set("User-Agent", UA)
	req.Header = header
	resp, err := http.DefaultClient.Do(&req)
	if err == nil {
		if resp.StatusCode == 200 {
			logger.Println(logfile, code+":sucess"+strconv.Itoa(resp.StatusCode))
			fmt.Println(code + ":sucess")
			//io.Copy(f, resp.Body)
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println("http read error")
			}
			src := string(body)
			f.WriteString(src)
		} else {
			logger.Println(logfile, code+":http get StatusCode"+strconv.Itoa(resp.StatusCode))
			fmt.Println(code + ":" + strconv.Itoa(resp.StatusCode))
		}
		defer resp.Body.Close()
	} else {
		logger.Println(logfile, code+":http get error"+code)
		fmt.Println(code + ":error")
	}
}

func isDirExists(path string) bool {
	fi, err := os.Stat(path)

	if err != nil {
		return os.IsExist(err)
	} else {
		return fi.IsDir()
	}
}

func show_usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [-d=<date>] [-n=<num>] [-s=<stockType>] [-t=<type>]\n"+
			"       <command> [<args>]\n\n",
		os.Args[0])
	fmt.Fprintf(os.Stderr,
		"Flags:\n")
	flag.PrintDefaults()
	/*
		fmt.Fprintf(os.Stderr,
			"\nCommands:\n"+
				"  autocomplete [<path>] <offset>     main autocompletion command\n"+
				"  close                              close the gocode daemon\n"+
				"  status                             gocode daemon status report\n"+
				"  drop-cache                         drop gocode daemon's cache\n"+
				"  set [<name> [<value>]]             list or set config options\n")
	*/
}
