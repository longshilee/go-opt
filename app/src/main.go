package main

import "github.com/go-martini/martini"
import "github.com/martini-contrib/render"

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	//"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	Dev  string = "development"
	Prod string = "production"
	Test string = "test"
)

const (
	test_pwd    string = "testtest"
	product_pwd string = "feihide"
)

//MARTINI_ENV=production

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type Env struct {
	Name   string
	Title  string
	Number int
	Pc     template.HTML
	Wechat template.HTML
}

type ResData struct {
	Name     string
	Return   string
	Ok       bool
	Duartion string
}

//var Env = Test

func main() {
	port := flag.String("port", "8400", "port number")
	flag.Parse()
	fmt.Println("启用端口:", *port)
	m := martini.Classic()
	m.Use(martini.Static("../assets"))
	m.Use(render.Renderer(render.Options{
		Directory: "../templates", // Specify what path to load the templates from.
		Layout:    "layout",       // Specify a layout template. Layouts can call {{ yield }} to render the current template.
		//		Extensions: []string{".tmpl", ".html"},  // Specify extensions to load for templates.
		//	Delims:     render.Delims{"{[{", "}]}"}, // Sets delimiters to the specified strings.
		Charset:    "UTF-8", // Sets encoding for json and html content-types. Default is "UTF-8".
		IndentJSON: true,    // Output human readable JSON
		IndentXML:  true,    // Output human readable XML
		//	HTMLContentType: "application/xhtml+xml",     // Output XHTML content type instead of default "text/html"
	}))
	m.Use(func(c martini.Context, log *log.Logger, res http.ResponseWriter, req *http.Request) {
		log.Println("before a request")

		fmt.Println(martini.Env)
		c.Next()

		log.Println("after a request")
	})
	m.Get("/", func() string {
		return "Hello world"
	})

	m.Get("/opt", func(r render.Render) {
		envs := []Env{{"dev", "开发环境", 1, "", ""}, {"test", "测试环境", 1, "", ""}, {"product", "生产环境", 2, "", ""}}
		logData, _ := ioutil.ReadFile("log.txt")
		checkUrl := map[string]string{"dev-pc": "http://devwww.kunlunhealth.com.cn", "dev-wechat": "http://devwechat.kunlunhealth.com.cn", "test-pc": "http://testwww.kunlunhealth.com.cn", "test-wechat": "http://testm.kunlunhealth.com.cn", "product-pc": "https://www.kunlunhealth.com.cn", "product-wechat": "https://m.kunlunhealth.com.cn"}
		result := make(chan string, 10)
		quit := make(chan int)
		//总并发超时时间
		//timeover := 10
		//http请求超时时间
		timeout := 10
		t := time.Duration(timeout) * time.Second
		Client := http.Client{Timeout: t}
		resultStatus := make(map[string]ResData)
		var Count int32
		Number := int32(0)
		for k, v := range checkUrl {
			go func(kk, vv string) {
				req, _ := http.NewRequest("GET", vv, nil)
				beginTime := time.Now()
				resp, er := Client.Do(req)

				//defer resp.Body.Close()
				var out ResData
				out = ResData{kk, "", false, time.Since(beginTime).String()}

				if er == nil && resp.StatusCode == 200 {
					b, _ := ioutil.ReadAll(resp.Body)
					html := "<!DOCTYPE html>"

					fmt.Println(string(b)[0:15])

					if string(b)[0:15] == html {
						out.Ok = true
						out.Return = string(b)[0:50]
					}
				}
				str, _ := json.Marshal(out)
				result <- string(str)
				atomic.AddInt32(&Count, int32(1)) //当所有地址请求完毕跳出循环
				fmt.Println("current:", Count, " finish:", kk)
				if Count == Number {
					quit <- 0
					close(result)
					close(quit)
				}
			}(k, v)
			Number++
		}
		fmt.Println("request all number", Number)

		for {
			select {
			case x := <-result:
				var getData ResData
				json.Unmarshal([]byte(x), &getData)
				resultStatus[getData.Name] = getData
			case <-quit:
				goto L
			}
		}
	L:
		for n, item := range envs {
			if resultStatus[item.Name+"-pc"].Ok {
				envs[n].Pc = template.HTML("<font color='green'>正常[耗时:" + resultStatus[item.Name+"-pc"].Duartion + "</font>")
			} else {
				envs[n].Pc = template.HTML("<font color='red'>异常[耗时:" + resultStatus[item.Name+"-pc"].Duartion + "</font>")
			}
			if resultStatus[item.Name+"-wechat"].Ok {
				content := "正常[耗时:" + resultStatus[item.Name+"-wechat"].Duartion
				envs[n].Wechat = template.HTML("<font color='green'>" + content + "</font>")
			} else {
				envs[n].Wechat = template.HTML("<font color='#FF0000'>异常[耗时:" + resultStatus[item.Name+"-wechat"].Duartion + "</font>")
			}
		}
		fmt.Println(resultStatus)
		r.HTML(200, "opt", map[string]interface{}{"envs": envs, "log": string(logData)})
	})
	m.Get("/opt/config", func(req *http.Request, r render.Render) {
		//queryForm, _ := url.ParseQuery(req.URL.RawQuery)
		name := req.FormValue("name")

		dat, err := ioutil.ReadFile("/work/kl/bin/" + name + "_export.cnf")
		check(err)
		r.JSON(200, map[string]interface{}{"result": string(dat)})
	})

	running := map[string]bool{"dev-front": false, "dev-java": false, "test-front": false, "test-java": false, "product-front": false, "proudct-java": false}
	m.Post("/opt/run", func(w http.ResponseWriter, req *http.Request, r render.Render) {
		//fmt.Fprintln(w, req.PostFormValue("name"))
		name := req.PostFormValue("name")
		pwd := req.PostFormValue("pwd")
		num, _ := strconv.Atoi(req.PostFormValue("num"))
		tmp := strings.Split(name, "-")
		isAllow := 0
		if tmp[0] == "dev" {
			isAllow = 1
		}
		if tmp[0] == "test" {
			if pwd == test_pwd {
				isAllow = 1
			}
		}
		if tmp[0] == "product" {
			if pwd == product_pwd {
				isAllow = 1
			}
		}

		var data string
		if isAllow == 0 {
			data = "无权更新"
		} else {
			if running[name] == false {
				running[name] = true
				fmt.Println(running)
				//command := "ls"
				//params := []string{"-l"}
				//执行cmd命令: ls -l
				command := "cd /work/kl/bin"
				if num == 1 {
					command += "&&./auto.sh " + req.PostFormValue("name") + " update"
				} else {
					for i := 1; i < num+1; i++ {
						command += "&&./auto.sh " + req.PostFormValue("name") + strconv.Itoa(i) + " update"
					}
				}
				//commandTest := "sleep 3&& echo 'fk'"
				ret, err := execCmd(command)
				fmt.Println(ret)
				if err != nil {
					fmt.Println(err)
					data = "更新失败"
				} else {
					data = "更新成功"
				}
				comm := "sed  -i \"1i\\ `date`  opt:" + name + " result:" + data + " \r\"  log.txt"
				//写入日志
				execCmd(comm)
				//run := "echo \" `date`  " + ret + " \"  >> runtime.txt"
				//execCmd(run)
				running[name] = false
			} else {
				fmt.Println("block")
				data = "执行更新中"
			}
		}
		r.JSON(200, map[string]interface{}{"result": data})
	})

	m.Get("/test/html", func(r render.Render) {
		r.HTML(200, "index", "fjfjf")
	})

	m.Get("/test/json", func(r render.Render) {
		r.JSON(200, map[string]interface{}{"hello": "world"})
	})

	m.Get("/test/text", func(r render.Render) {
		r.Text(200, "hello, world")
	})
	m.Get("/readfile", func() string {
		dat, err := ioutil.ReadFile("/work/test.pdf")
		check(err)
		return string(dat)
	})

	//路由分组
	m.Group("/books", func(r martini.Router) {
		r.Get("/list", getBooks)
		r.Post("/add", getBooks)
		r.Delete("/delete", getBooks)
	})

	m.Get("/test", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(200)
	})
	m.Get("/hello/:name", func(params martini.Params) string {
		return "Hello " + params["name"]
	})
	m.NotFound(func(res http.ResponseWriter) string {
		res.WriteHeader(404)
		return "noFound"
	})

	m.RunOnAddr(":" + *port)
}

func execCmd(command string) (string, error) {

	cmd := exec.Command("sh", "-c", command)
	fmt.Println("exec:", cmd.Args)
	buf, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "The command failed to perform: %s (Command: %s)", err, command)
		return "", err
	}
	//fmt.Fprintf(os.Stdout, "Result: %s", buf)
	return string(buf), nil
}

func execCommand(commandName string, params []string) bool {
	cmd := exec.Command(commandName, params...)

	//显示运行的命令
	fmt.Println(cmd.Args)

	stdout, err := cmd.StdoutPipe()

	if err != nil {
		fmt.Println(err)
		return false
	}

	cmd.Start()

	reader := bufio.NewReader(stdout)
	//实时循环读取输出流中的一行内容
	for {
		line, err2 := reader.ReadString('\n')
		if err2 != nil || io.EOF == err2 {
			break
		}
		fmt.Println(line)
	}

	cmd.Wait()
	return true
}

func getBooks() string {
	return "books"
}
