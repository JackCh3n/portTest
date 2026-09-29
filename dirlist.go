package main

import (
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
)

// makeStaticFileHandler 包装 http.FileServer:
//   - 文件请求原样交给 FileServer(保留 Range 断点续传/HEAD/缓存协商)
//   - 目录请求渲染 nginx autoindex 风格的极简列表页(替代 FileServer 默认样式)
//
// urlPrefix 为该处理器对外暴露的 URL 前缀(根挂载为 "", /static 挂载为 "/static"),
// 仅用于页面标题显示; 列表内链接使用相对路径, 嵌套任意前缀均正确。
func makeStaticFileHandler(dirFS http.FileSystem, urlPrefix string) http.Handler {
	fileServer := http.FileServer(dirFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 统一 nosniff: 列表页与文件下载均禁止 MIME 嗅探
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !strings.HasSuffix(r.URL.Path, "/") {
			// 文件(或由 FileServer 做斜杠重定向的路径): 原样转发
			fileServer.ServeHTTP(w, r)
			return
		}
		// 目录: 渲染列表页, 打不开时回退 FileServer 的默认行为(403/404)
		upath := path.Clean("/" + r.URL.Path)
		f, err := dirFS.Open(upath)
		if err != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		d, ok := f.(fs.ReadDirFile)
		if !ok {
			fileServer.ServeHTTP(w, r)
			return
		}
		entries, err := d.ReadDir(-1)
		if err != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		renderDirList(w, urlPrefix, upath, entries)
	})
}

// renderDirList 输出 nginx autoindex 风格的极简目录列表页
func renderDirList(w http.ResponseWriter, urlPrefix, dirPath string, entries []fs.DirEntry) {
	// 目录在前, 同类按名称排序
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Index of `)
	b.WriteString(html.EscapeString(dirPath))
	b.WriteString(`</title>
<style>
body{font-family:-apple-system,"Segoe UI",Roboto,"PingFang SC","Microsoft YaHei",sans-serif;max-width:860px;margin:40px auto;padding:0 16px;color:#24292f;background:#fff}
h1{font-size:15px;font-weight:600;color:#57606a;border-bottom:1px solid #d8dee4;padding-bottom:10px;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;word-break:break-all;margin:0 0 12px}
table{width:100%;border-collapse:collapse;font-size:14px}
th{font-weight:400;color:#8b949e;text-align:left;padding:6px 8px;border-bottom:1px solid #d8dee4}
td{padding:6px 8px;border-bottom:1px solid #f6f8fa;white-space:nowrap}
td.name{white-space:normal;word-break:break-all}
.num{text-align:right;color:#57606a;font-variant-numeric:tabular-nums}
a{color:#0969da;text-decoration:none}
a:hover{text-decoration:underline}
a.dir::after{content:"/";color:#8b949e}
footer{margin-top:16px;font-size:12px;color:#8b949e}
.ops{white-space:nowrap;text-align:right;width:170px}
.ops button{font:12px/1 ui-monospace,SFMono-Regular,Consolas,monospace;color:#57606a;background:#f6f8fa;border:1px solid #d8dee4;border-radius:4px;padding:2px 7px;cursor:pointer;margin-left:4px}
.ops button:hover{color:#0969da;border-color:#0969da;background:#fff}
.ops button.ok{color:#1a7f37;border-color:#1a7f37}
</style>
</head>
<body>
<h1>Index of `)
	b.WriteString(html.EscapeString(urlPrefix + dirPath))
	b.WriteString(`</h1>
<table>
<tr><th style="width:50%">Name</th><th class="num">Size</th><th class="num" style="width:145px">Modified</th><th class="ops">Copy</th></tr>
`)
	// 非根目录显示返回上级
	if dirPath != "/" {
		b.WriteString(`<tr><td class="name"><a href="../">&#8592; ..</a></td><td class="num">-</td><td class="num">-</td></tr>`)
	}
	for _, e := range entries {
		name := e.Name()
		href := url.PathEscape(name)
		if e.IsDir() {
			href += "/"
		}
		size, mod := "-", ""
		if info, err := e.Info(); err == nil {
			if !e.IsDir() {
				size = formatBytes(info.Size())
			}
			mod = info.ModTime().Format("2006-01-02 15:04")
		}
		b.WriteString(`<tr><td class="name">`)
		b.WriteString(fileIcon(name, e.IsDir()))
		b.WriteString(` <a class="`)
		if e.IsDir() {
			b.WriteString("dir")
		}
		b.WriteString(`" href="`)
		b.WriteString(href)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(name))
		b.WriteString(`</a></td><td class="num">`)
		b.WriteString(size)
		b.WriteString(`</td><td class="num">`)
		b.WriteString(mod)
		b.WriteString(`</td>`)
		if e.IsDir() {
			b.WriteString(`<td class="ops"></td>`)
		} else {
			// 一键复制: curl 下载 / wget 下载 / 下载并运行; 命令由前端按当前访问地址生成
			b.WriteString(`<td class="ops">`)
			for _, k := range []string{"curl", "wget", "run"} {
				b.WriteString(`<button type="button" title="复制 `)
				switch k {
				case "curl":
					b.WriteString(`curl 下载命令`)
				case "wget":
					b.WriteString(`wget 下载命令`)
				case "run":
					b.WriteString(`下载并运行命令(curl+chmod+执行)`)
				}
				b.WriteString(`" onclick="cp(this,'`)
				b.WriteString(k)
				b.WriteString(`')">`)
				b.WriteString(k)
				b.WriteString("</button>")
			}
			b.WriteString(`</td>`)
		}
		b.WriteString("</tr>")
	}
	b.WriteString(`</table>
<footer>port-test v`)
	b.WriteString(version)
	b.WriteString(` download station</footer>
<script>
function shq(s){return "'"+String(s).replace(/'/g,"'\\''")+"'";}
function doCopy(t){
  if(navigator.clipboard&&window.isSecureContext){return navigator.clipboard.writeText(t);}
  var ta=document.createElement('textarea');
  ta.value=t;ta.style.position='fixed';ta.style.opacity='0';
  document.body.appendChild(ta);ta.focus();ta.select();
  var ok=false;try{ok=document.execCommand('copy');}catch(e){}
  document.body.removeChild(ta);
  return ok?Promise.resolve():Promise.reject();
}
function absUrl(href){
  try{return new URL(href,location.href).href;}catch(e){}
  var o=location.protocol+'//'+location.host;
  if(href.charAt(0)==='/')return o+href;
  return o+location.pathname.replace(/[^\/]*$/,'')+href;
}
function cp(btn,kind){
  var a=btn.closest('tr').querySelector('td.name a');
  var raw=a.getAttribute('href').split('/').pop();
  var name;try{name=decodeURIComponent(raw);}catch(e){name=raw;}
  var url=absUrl(a.getAttribute('href'));
  var q=shq,cmd;
  if(kind==='curl')cmd='curl -f -o '+q(name)+' '+q(url);
  else if(kind==='wget')cmd='wget -O '+q(name)+' '+q(url);
  else cmd='curl -f -o '+q(name)+' '+q(url)+' && chmod +x '+q(name)+' && '+q('./'+name);
  doCopy(cmd).then(function(){done(btn);},function(){prompt('复制失败, 请手动复制:',cmd);});
}
function done(btn){
  var t=btn.textContent;btn.textContent='✓';btn.classList.add('ok');
  setTimeout(function(){btn.textContent=t;btn.classList.remove('ok');},1200);
}
</script>
</body>
</html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write([]byte(b.String()))
}

// formatBytes 字节数人性化显示 (B/KB/MB/GB...)
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// fileIcon 按文件后缀返回 emoji 图标, 纯文本实现无需额外资源
func fileIcon(name string, isDir bool) string {
	if isDir {
		return "📁"
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico":
		return "🖼️"
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return "🎬"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a", ".wma":
		return "🎵"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz", ".iso", ".img":
		return "📦"
	case ".pdf":
		return "📕"
	case ".doc", ".docx":
		return "📘"
	case ".xls", ".xlsx", ".csv":
		return "📊"
	case ".ppt", ".pptx":
		return "📙"
	case ".exe", ".msi", ".apk", ".dmg", ".deb", ".rpm", ".appimage":
		return "⚙️"
	case ".bin", ".so", ".dll", ".dylib":
		return "🔧"
	case ".go", ".py", ".js", ".ts", ".c", ".h", ".cpp", ".java", ".rs", ".sh", ".bat", ".ps1":
		return "💻"
	case ".html", ".css", ".xml", ".json", ".yml", ".yaml", ".toml", ".ini", ".conf":
		return "🧾"
	case ".md", ".txt", ".log":
		return "📝"
	case ".sql", ".db", ".sqlite":
		return "🗄️"
	default:
		return "📄"
	}
}
