package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const proxyPoolPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Proxy Pool Dashboard</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#faf7f5;color:#333;min-height:100vh}
.header{background:#fff;border-bottom:1px solid #e8e0d8;padding:16px 24px;display:flex;align-items:center;gap:16px;box-shadow:0 1px 3px rgba(0,0,0,0.04)}
.header h1{font-size:18px;font-weight:600;color:#1a1a1a}
.header .back{text-decoration:none;color:#8b7355;font-size:14px;padding:6px 12px;border:1px solid #d4c5b0;border-radius:6px;transition:all .2s}
.header .back:hover{background:#f0e8dd;color:#6b5640}
.header .status{margin-left:auto;display:flex;align-items:center;gap:8px;font-size:13px;color:#666}
.header .dot{width:8px;height:8px;border-radius:50%;display:inline-block}
.dot.on{background:#22c55e;box-shadow:0 0 6px rgba(34,197,94,0.4)}
.dot.off{background:#ef4444;box-shadow:0 0 6px rgba(239,68,68,0.3)}
.container{max-width:1000px;margin:24px auto;padding:0 24px}
.card{background:#fff;border-radius:12px;border:1px solid #e8e0d8;padding:20px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.04)}
.card h2{font-size:15px;font-weight:600;margin-bottom:16px;color:#4a3f35;display:flex;align-items:center;gap:8px}
.card h2 .icon{font-size:18px}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-bottom:16px}
.stat{background:#faf7f5;border-radius:8px;padding:14px;text-align:center;border:1px solid #ede5db}
.stat .val{font-size:22px;font-weight:700;color:#1a1a1a}
.stat .label{font-size:11px;color:#8b7355;margin-top:4px;text-transform:uppercase;letter-spacing:.5px}
table{width:100%;border-collapse:separate;border-spacing:0;font-size:13px}
th{text-align:left;padding:10px 12px;background:#faf7f5;color:#8b7355;font-weight:600;font-size:11px;text-transform:uppercase;letter-spacing:.5px;border-bottom:2px solid #e8e0d8}
td{padding:10px 12px;border-bottom:1px solid #f0e8dd;vertical-align:middle}
tr:hover td{background:#fdfbf9}
.badge{display:inline-flex;align-items:center;gap:5px;padding:3px 10px;border-radius:12px;font-size:11px;font-weight:600}
.badge.healthy{background:#dcfce7;color:#166534}
.badge.unhealthy{background:#fee2e2;color:#991b1b}
.config-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:12px}
.config-item{background:#faf7f5;border-radius:8px;padding:12px;border:1px solid #ede5db}
.config-item .key{font-size:11px;color:#8b7355;text-transform:uppercase;letter-spacing:.5px;margin-bottom:4px}
.config-item .value{font-size:14px;font-weight:600;color:#1a1a1a}
.empty{text-align:center;padding:48px 24px;color:#999}
.empty .icon{font-size:36px;margin-bottom:12px}
.empty p{font-size:14px;line-height:1.6}
.actions{display:flex;gap:8px;margin-bottom:16px}
.btn{padding:8px 16px;border-radius:8px;border:1px solid #d4c5b0;background:#fff;color:#4a3f35;font-size:13px;cursor:pointer;transition:all .2s;font-weight:500}
.btn:hover{background:#f0e8dd}
.btn.primary{background:#8b7355;color:#fff;border-color:#8b7355}
.btn.primary:hover{background:#6b5640}
.btn.sm{padding:4px 10px;font-size:12px}
.timer{font-size:12px;color:#999;margin-left:auto}
.url-text{font-family:'SF Mono',Monaco,Consolas,monospace;font-size:12px;color:#4a3f35;word-break:break-all}
.latency{font-family:'SF Mono',Monaco,Consolas,monospace;font-size:12px}
.latency.fast{color:#16a34a}
.latency.medium{color:#ca8a04}
.latency.slow{color:#dc2626}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
.loading{animation:pulse 1.5s infinite}
</style>
</head>
<body>
<div class="header">
<a href="/management.html" class="back">← 管理面板</a>
<h1>🔄 代理池监控</h1>
<div class="status" id="connStatus">
<span class="dot off" id="statusDot"></span>
<span id="statusText">加载中...</span>
</div>
</div>
<div class="container">
<div class="card">
<h2><span class="icon">📊</span> 概览</h2>
<div class="stats" id="statsGrid">
<div class="stat"><div class="val loading" id="totalCount">-</div><div class="label">代理总数</div></div>
<div class="stat"><div class="val loading" id="healthyCount">-</div><div class="label">健康</div></div>
<div class="stat"><div class="val loading" id="unhealthyCount">-</div><div class="label">不健康</div></div>
<div class="stat"><div class="val loading" id="strategyVal">-</div><div class="label">策略</div></div>
</div>
</div>
<div class="card">
<h2><span class="icon">🖧</span> 代理列表</h2>
<div class="actions">
<button class="btn primary" onclick="refresh()">🔄 刷新</button>
<label><input type="checkbox" id="autoRefresh" checked onchange="toggleAuto()"> 自动刷新 (10s)</label>
<span class="timer" id="timerText"></span>
</div>
<div id="proxyTable"></div>
</div>
<div class="card">
<h2><span class="icon">⚙️</span> 配置</h2>
<div class="config-grid" id="configGrid"></div>
</div>
</div>
<script>
const API_BASE=window.location.origin;
let autoTimer=null;
let countdown=10;
let mgmtKey='';
let authRetries=0;

// Try to read key from management panel's storage formats
function loadKey(){
  // Try our own storage first
  let k=localStorage.getItem('proxy-pool-key')||'';
  if(k)return k;
  // Try management panel's obfuscated format (enc::v1::base64)
  for(let i=0;i<localStorage.length;i++){
    const key=localStorage.key(i);
    const val=localStorage.getItem(key);
    if(val&&val.startsWith('enc::v1::')){
      try{return atob(val.replace('enc::v1::',''));}catch(e){}
    }
  }
  // Try common key names
  const tryKeys=['management-key','mgmt-key','secret-key','api-management-key'];
  for(const tk of tryKeys){
    const v=localStorage.getItem(tk);
    if(v&&!v.startsWith('enc::'))return v;
  }
  return '';
}

mgmtKey=loadKey();
if(!mgmtKey){
  mgmtKey=prompt('请输入管理密钥 (Management Key):','');
  if(mgmtKey)localStorage.setItem('proxy-pool-key',mgmtKey);
}

function headers(){
  const h={'Content-Type':'application/json'};
  if(mgmtKey)h['Authorization']='Bearer '+mgmtKey;
  return h;
}

async function fetchPool(){
  try{
    const r=await fetch(API_BASE+'/v0/management/proxy-pool',{headers:headers()});
    if(r.status===401||r.status===403){
      if(authRetries<2){
        authRetries++;
        localStorage.removeItem('proxy-pool-key');
        localStorage.removeItem('management-key');
        mgmtKey=prompt('管理密钥无效('+r.status+')，请重新输入:','');
        if(mgmtKey){localStorage.setItem('proxy-pool-key',mgmtKey);return fetchPool();}
      }
      return null;
    }
    authRetries=0;
    return await r.json();
  }catch(e){
    console.error('fetch error:',e);
    return null;
  }
}

function renderStats(d){
  if(!d){
    document.getElementById('statusDot').className='dot off';
    document.getElementById('statusText').textContent='连接失败';
    return;
  }
  const entries=d.entries||[];
  const healthy=entries.filter(e=>e.healthy).length;
  const unhealthy=entries.length-healthy;
  document.getElementById('totalCount').textContent=entries.length;
  document.getElementById('totalCount').classList.remove('loading');
  document.getElementById('healthyCount').textContent=healthy;
  document.getElementById('healthyCount').classList.remove('loading');
  document.getElementById('unhealthyCount').textContent=unhealthy;
  document.getElementById('unhealthyCount').classList.remove('loading');
  const s=d.config?.strategy||'round-robin';
  document.getElementById('strategyVal').textContent=s;
  document.getElementById('strategyVal').classList.remove('loading');
  document.getElementById('statusDot').className='dot '+(d.enabled?'on':'off');
  document.getElementById('statusText').textContent=d.enabled?'已启用 ('+entries.length+' 个代理)':'未启用';
}

function renderTable(d){
  const el=document.getElementById('proxyTable');
  if(!d||!d.entries||d.entries.length===0){
    el.innerHTML='<div class="empty"><div class="icon">📭</div><p>代理池未配置<br>在 config.yaml 中添加 <code>proxy-urls</code> 列表</p></div>';
    return;
  }
  let html='<table><tr><th>代理 URL</th><th>状态</th><th>延迟</th><th>失败次数</th><th>最后检查</th></tr>';
  d.entries.forEach(e=>{
    const badge=e.healthy?'<span class="badge healthy"><span class="dot on"></span>健康</span>':'<span class="badge unhealthy"><span class="dot off"></span>不健康</span>';
    let latCls='fast';
    if(e.latency){
      const ms=parseFloat(e.latency);
      if(ms>1000)latCls='slow';else if(ms>500)latCls='medium';
    }
    const lat=e.latency?'<span class="latency '+latCls+'">'+e.latency+'</span>':'-';
    html+='<tr><td class="url-text">'+escHtml(e.url)+'</td><td>'+badge+'</td><td>'+lat+'</td><td>'+e.failure_count+'</td><td>'+(e.last_check||'未检查')+'</td></tr>';
  });
  html+='</table>';
  el.innerHTML=html;
}

function renderConfig(d){
  const el=document.getElementById('configGrid');
  if(!d||!d.config){el.innerHTML='';return;}
  const c=d.config;
  el.innerHTML=[
    item('策略',c.strategy||'round-robin'),
    item('健康检查间隔',(c.health_check_interval||60)+'s'),
    item('检查 URL',c.health_check_url||'chatgpt.com/favicon.ico'),
    item('最大失败次数',c.max_failures||3)
  ].join('');
}

function item(k,v){return '<div class="config-item"><div class="key">'+k+'</div><div class="value">'+escHtml(String(v))+'</div></div>';}
function escHtml(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}

async function refresh(){
  const d=await fetchPool();
  renderStats(d);renderTable(d);renderConfig(d);
}

function toggleAuto(){
  if(document.getElementById('autoRefresh').checked){startAuto();}
  else{if(autoTimer)clearInterval(autoTimer);autoTimer=null;document.getElementById('timerText').textContent='';}
}

function startAuto(){
  if(autoTimer)clearInterval(autoTimer);
  countdown=10;
  autoTimer=setInterval(()=>{
    countdown--;
    document.getElementById('timerText').textContent='下次刷新: '+countdown+'s';
    if(countdown<=0){countdown=10;refresh();}
  },1000);
}

refresh();
if(document.getElementById('autoRefresh').checked)startAuto();
</script>
</body>
</html>`

// ServeProxyPoolPage redirects to the integrated proxy pool view in the management panel.
func (h *Handler) ServeProxyPoolPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/management.html#/proxy-pool")
}
