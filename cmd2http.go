package main


import (
	"fmt"
	"log"
	 "net/http"
	 "flag"
	 "os"
	 "bytes"
	 "time"
	 "strings"
	 "io/ioutil"
	 "regexp"
	 "encoding/json"
	 "os/exec"
	 "mime"
	 "strconv"
	 "path/filepath"
	 "text/template"
	 jsonConf "github.com/daviddengcn/go-ljson-conf"
	  "github.com/cookieo9/resources-go/v2/resources"
   )
var configPath=flag.String("conf","./cmd2http.conf","config file")

var port int
var version string

type param struct{
  name string
  defaultValue string
  isValParam bool
}

type Conf struct{
   name string
   cmdStr string
   cmd string
   charset string
   params []*param
   intro string
   timeout int
}

var confMap map[string]*Conf

var config *jsonConf.Conf

func main(){
   flag.Parse()
//    log.SetFlags(log.LstdFlags|log.Lshortfile)
    loadConfig()
   logFile,_:=os.OpenFile("./cmd2http.log",os.O_CREATE|os.O_RDWR|os.O_APPEND,0666)
   defer logFile.Close()
   log.SetOutput(logFile)
    
    startHttpServer()
}

func startHttpServer(){
//   http.ReadTimeout=60 * time.Second
   
   http.Handle("/s/",http.FileServer(http.Dir("./")))
   http.HandleFunc("/res/",myHandler_res)
   http.HandleFunc("/",myHandler_root)
   http.HandleFunc("/help",myHandler_help)
   
   addr:=fmt.Sprintf(":%d",port)
   log.Println("listen at",addr)
   fmt.Println("listen at",addr)
   
   http.ListenAndServe(addr,nil)
}

func (p *param)ToString() string{
    return fmt.Sprintf("name:%s,default:%s,isValParam:%x",p.name,p.defaultValue,p.isValParam);
}
func loadConfig(){
   version=getVersion()

   log.Println("start load conf [",*configPath,"]")
   log.Println("use conf:",*configPath)
   
   pathAbs,_:=filepath.Abs(*configPath)
   
   _,_err:= os.Open(pathAbs)
   if _err != nil {
        panic(_err.Error())
    }
   os.Chdir(filepath.Dir(*configPath))
    
   var err error
   config, err= jsonConf.Load(*configPath)
	if err != nil {
	  log.Println(err.Error(),config)
	  os.Exit(2)
	}
	port=config.Int("port",8310)
	
	timeout:=config.Int("timeout",30)
	if(timeout<1){
	  timeout=1
	}
	
	confMap=make(map[string]*Conf)
	
	cmds:=config.Object("cmds",make(map[string]interface{}))
	
	for k,v:=range cmds{
	   _conf:=v.(map[string]interface{})
	   conf:=new(Conf)
	   conf.name=k
	   conf.timeout=timeout
	   conf.charset="utf-8"
	  if _charset,_has:=_conf["charset"];_has { 
	      conf.charset=_charset.(string)
	    }
	   if _intro,_has:=_conf["intro"];_has{
	     conf.intro=_intro.(string)
	   }
	   if _timeout,_has:=_conf["timeout"];_has{
	       t:=0;
	        switch v:= _timeout.(type){
	            case string:
	                i, err := strconv.ParseInt(v, 0, 0)
	                if err == nil {
	                     t=int(i)
	                     }
                case float64:
                      t=int(_timeout.(float64))
	          }
	          
	     if(t>0){
   	      conf.timeout=t
	       }
	   }
	   conf.cmdStr,_=_conf["cmd"].(string)
	   conf.cmdStr=strings.TrimSpace(conf.cmdStr)
	   conf.params=make([]*param,0,10)
	   
	   ps:=regexp.MustCompile(`\s+`).Split(conf.cmdStr,-1)
//	   fmt.Println(ps)
	   conf.cmd=ps[0]
	   
	   for i:=1;i<len(ps);i++ {
	       item:=ps[i]
//	       fmt.Println("i:",i,item)
	        _param:=new(param)
	        _param.name=item
	       
	       if(item[0]=='$'){
	        _param.isValParam=true;
	        tmp:=strings.Split(item+"|","|")
	        _param.name=tmp[0][1:]
	        _param.defaultValue=tmp[1]
	       }
	       conf.params=append(conf.params,_param)
//	       fmt.Println(_param.name,_param.defaultValue)
	    }
	   log.Println("register[",k,"] cmd:",conf.cmdStr)
	   confMap[k]=conf
	}
	
	log.Println("load conf [",*configPath,"] finish [ok]")
}

func loadRes(path string) []byte{
    path=strings.TrimLeft(path,"/")
    res,err:=resources.Find(path)
    if(err!=nil){
      log.Println("load res[",path,"] failed",err.Error())
      return []byte{};
     }
     r,_:=res.Open()
     bf,err:=ioutil.ReadAll(r)
     if(err!=nil){
        log.Println("read res[",path,"] failed",err.Error())
      }
     return bf
}



func Command(name string, args []string) *exec.Cmd {
	aname, err := exec.LookPath(name)
	if err != nil {
		aname = name
	}
	return &exec.Cmd{
		Path: aname,
		Args: args,
	}
}

func isFileExists(path string) bool{
  _,err := os.Stat( path )
  return err==nil
}

func myHandler_root(w http.ResponseWriter, r *http.Request){
     startTime:=time.Now()
	  path:=strings.Trim(r.URL.Path,"/")
	  if(path==""){
			if isFileExists("./s/index.html") {
		     http.Redirect(w,r,"/s/",302)
			  return;
			}
	      myHandler_help(w,r)
	      return;
	   }
	   
	  logStr:=r.RemoteAddr+" req:"+r.RequestURI+" "
	  defer func(){
	       logStr+=fmt.Sprintf(" time_use:%v",time.Now().Sub(startTime))
	       log.Println(logStr)
	   }()
	  
	  conf,has:=confMap[path]
	  if(!has) {
	     logStr=logStr+"not support cmd"
	     fmt.Fprintf(w,"not support")
	     return;
	  }
	  
	  args:=make([]string,len(conf.params)+1)
	  for i,_param:=range conf.params{
		  if(!_param.isValParam){
	        args[i+1]=_param.name
		     continue
		  }
	     val:=r.FormValue(_param.name)
	     if(val==""){
	        val=_param.defaultValue
	      }
	      args[i+1]=val
	  }
	  cmd := Command(conf.cmd,args)
  	  var out bytes.Buffer
		cmd.Stdout = &out
	   err:=cmd.Start()
	 

	  if(err!=nil){
	     logStr+=err.Error()
	     fmt.Fprintf(w,err.Error())
	     return;
	   }
   	done := make(chan error)
		go func() {
		    done <- cmd.Wait()
		}()
		
		cc := w.(http.CloseNotifier).CloseNotify()
		
		isResonseOk:=true
		
		killCmd:=func(msg string){
			  if err := cmd.Process.Kill(); err != nil {
	            log.Println("failed to kill: ", err)
	           }
	        logStr+="[killed:"+msg+"]"
//	        log.Println(logStr)
	        isResonseOk=false
		}
		
		select {
		    case <-cc:
		         killCmd("client close")
		    case <-time.After(time.Duration(conf.timeout) * time.Second):
               killCmd("timeout")
          case <-done:
                  
		}
	  
		if(!isResonseOk){
		    return;
		}
		
	  format:=r.FormValue("format")
	  str:=`<!DOCTYPE html><html><head>
	         <meta http-equiv='Content-Type' content='text/html; charset=%s' />
	         <title>%s cmd2http</title></head><body><pre>%s</pre></body></html>`
	         
	  outStr:=out.String()
	  logStr+=fmt.Sprintf("resLen:%d ",len(outStr))
	  
	  if(format=="" || format=="html"){
	       w.Header().Set("Content-Type","text/html;charset="+conf.charset)
	    fmt.Fprintf(w,str,conf.charset,conf.name,outStr)
	   }else if(format=="jsonp"){
	       cb:=r.FormValue("cb")
	       if(cb==""){
	           cb="cb"
	        }
	       m:=make(map[string]string)
	       m["data"]=outStr
	       jsonByte,_:=json.Marshal(m)
	       fmt.Fprintf(w,fmt.Sprintf(`%s(%s)`,cb,string(jsonByte)))
	   }else{ 
	    w.Write([]byte(outStr))
	   }
}


func myHandler_res(w http.ResponseWriter, r *http.Request){
   mimeType:= mime.TypeByExtension(filepath.Ext(r.URL.Path))
   if(mimeType!=""){
       w.Header().Set("Content-Type",mimeType)
     }
    w.Write(loadRes(r.URL.Path))
}

func myHandler_help(w http.ResponseWriter, r *http.Request){
       str:=string(loadRes("res/tpl/help.html"));
       tabs_hd:="<div class='jw-tab'><div class='hd'><ul>\n";
       tabs_bd:="<div class='bd'>";
       for name,_conf:=range confMap{
           tabs_hd+="<li><a>"+name+"</a></li>"
           tabs_bd+="\n\n<div>\n<form action='/"+name+"' methor='get' onsubmit='return form_check(this,\""+name+"\")' id='form_"+name+"'>\n";
           tabs_bd+="<div class='note'><div><b>command</b> :&nbsp;[&nbsp;"+_conf.cmdStr+
           "&nbsp;]&nbsp;<b>timeout</b> :&nbsp;"+fmt.Sprintf("%d",_conf.timeout)+"s</div>"
           if(_conf.intro!=""){
             tabs_bd=tabs_bd+"<div><b>intro</b> :&nbsp;&nbsp;"+_conf.intro+"</div>"
              }
           tabs_bd=tabs_bd+"</div>";
           tabs_bd=tabs_bd+"<fieldset><ul class='ul-1'>"
              for _,_param:=range _conf.params{
                if(_param.isValParam){
                   placeholder:=""
                   if(_param.defaultValue!=""){
                      placeholder="placeholder='"+_param.defaultValue+"'"
                         }
                   tabs_bd=tabs_bd+"<li>"+_param.name+
                   ":<input class='r-text p_"+_param.name+"' type='text' name='"+_param.name+"' "+placeholder+"></li>\n";
                     }
                   }
           tabs_bd=tabs_bd+"</ul><input type='submit'>&nbsp;<input type='reset'></fieldset><br/><div class='div_url'></div>"+
            "<iframe src='about:_blank' style='border:none;width:99%;height:10px' onload='ifr_load(this)'></iframe>"+
            "</form>\n</div>\n\n";
          }
        
      tabs_str:=tabs_hd+"</ul></div>"+tabs_bd+"</div></div>";
      if(isFileExists("./s/my.css")){
        tabs_str+="<link  type='text/css' rel='stylesheet' href='/s/my.css'>";
        }
      
      if(isFileExists("./s/my.js")){
        tabs_str+="<script src='/s/my.js'></script>";
        }
      
	   title:=config.String("title","")
	   
	   reg:=regexp.MustCompile(`\s+`)
	   tabs_str=reg.ReplaceAllString(tabs_str," ")
	   
	   str=reg.ReplaceAllString(str," ")
	   
	   tpl,_:=template.New("page").Parse(str)
	   values :=make(map[string]string)
	   values["version"]=version
	   values["title"]=title
	   values["form_tabs"]=tabs_str
	   values["intro"]=config.String("intro","")
	   
	   
	   w.Header().Add("c2h",version)
	   tpl.Execute(w,values)
}

func getVersion() string{
   return string(loadRes("res/version"));
}
