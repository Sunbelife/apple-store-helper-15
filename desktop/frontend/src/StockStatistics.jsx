import React, { useEffect, useState } from "react";
import { X, RefreshCw } from "lucide-react";
const today = () => new Intl.DateTimeFormat("en-CA", {timeZone:"Asia/Shanghai",year:"numeric",month:"2-digit",day:"2-digit"}).format(new Date());
const areaNames = {cn:"中国大陆",hk:"香港",jp:"日本",sg:"新加坡",us:"美国",uk:"英国",au:"澳大利亚"};
export default function StockStatistics({onClose, load}) {
 const [filters,setFilters] = useState({date:today(),area:"",model:"",capacity:"",color:"",city:"",recent:""});
 const [data,setData]=useState(null),[busy,setBusy]=useState(true),[error,setError]=useState(""),[refresh,setRefresh]=useState(0);
 useEffect(()=>{const key=e=>{if(e.key==="Escape")onClose()};document.addEventListener("keydown",key);return()=>document.removeEventListener("keydown",key)},[onClose]);
 useEffect(()=>{let active=true;setBusy(true);setError("");load(filters).then(d=>{if(active)setData(d)}).catch(e=>{if(active){setData(null);setError(String(e))}}).finally(()=>{if(active)setBusy(false)});return()=>{active=false}},[filters,refresh]);
 useEffect(()=>{const timer=setInterval(()=>setRefresh(x=>x+1),300000);return()=>clearInterval(timer)},[]);
 const change=(key,value)=>setFilters(f=>({...f,[key]:value,...(key!=="city"?{city:""}:{})}));
 const max=Math.max(1,...(data?.hourly||[])), top=data?.hourly?.indexOf(Math.max(...data.hourly));
 return <div className="overlay" onClick={onClose}><section className="stats-dialog" role="dialog" aria-modal="true" aria-labelledby="stats-title" onClick={e=>e.stopPropagation()}>
  <div className="stats-heading"><div><div className="eyebrow">来自博客库存上报 · 北京时间</div><h2 id="stats-title">库存统计</h2></div><div className="stats-actions"><button className="text-button" disabled={busy} onClick={()=>setRefresh(x=>x+1)}><RefreshCw size={14} className={busy?"spin":""}/>刷新</button><button autoFocus className="icon-button" aria-label="关闭库存统计" onClick={onClose}><X size={20}/></button></div></div>
  <div className="stats-filters"><label>日期<input aria-label="统计日期" type="date" value={filters.date} max={today()} onChange={e=>{if(e.target.value)change("date",e.target.value)}}/></label>{[["model","型号"],["area","地区"],["color","颜色"],["capacity","容量"]].map(([key,label])=><label key={key}>{label}<select aria-label={`统计${label}`} value={filters[key]} onChange={e=>change(key,e.target.value)}><option value="">全部{label}</option>{data?.options?.[key]?.map(v=><option key={v} value={v}>{key==="area"?areaNames[v]||v:v}</option>)}</select></label>)}</div>
  <div className="stats-period"><button className={!filters.recent?"selected":""} onClick={()=>change("recent","")}>当日累计</button><button className={filters.recent?"selected":""} onClick={()=>change("recent","1")}>最近 1 小时</button>{filters.city&&<button onClick={()=>change("city","")}>{filters.city} ×</button>}</div>
  {busy?<p className="stats-empty" role="status">正在载入统计…</p>:error?<p className="stats-empty" role="alert">{error}</p>:data&&<>
   <div className="stats-metrics"><div><strong>{data.total}</strong><span>条有货检测记录</span></div><div><strong>{data.city_count}</strong><span>个城市有记录</span></div><p>{data.total?`${top} 点记录最多` : "所选范围暂无记录"}</p></div>
   <div className="stats-chart" aria-label="每小时检测记录数量">{data.hourly.map((v,i)=><button type="button" className={"stats-hour " + (i < 4 ? "tooltip-left" : i > 19 ? "tooltip-right" : "")} key={i} aria-label={`${i} 点至 ${i+1} 点：${v} 条，查看预览`}>
    <div className="stats-hour-bar" style={{height:`${v/max*88}px`}}/>
    <span className="stats-hour-label">{i%4===0?i+"时":""}</span>
    <span className="stats-hour-preview" role="tooltip"><strong>{String(i).padStart(2,"0")}:00–{String(i+1).padStart(2,"0")}:00 · {v} 条</strong>{v ? (data.hourly_products?.[i] || []).map(p=><span className="stats-preview-product" key={p.model+p.color+p.capacity}><b>{p.model}</b><span>{p.color} · {p.capacity} · {p.count} 条</span></span>) : <span>该小时暂无有货检测记录</span>}</span>
   </button>)}</div>
   {data.products?.length > 0 && <><div className="stats-ranking-title"><h3>检测到的机型</h3><span>型号 · 颜色 · 容量</span></div><div className="stats-products">{data.products.map(p=><div className="stats-product" key={p.model+p.color+p.capacity}><strong>{p.model}</strong><span>{p.color} · {p.capacity}</span><b>{p.count} 条</b></div>)}</div></>}
   <div className="stats-ranking-title"><h3>城市排行</h3><span>点击城市筛选统计</span></div>
   {data.cities.length?data.cities.map((c,i)=><button className="stats-city" key={c.area+c.city} onClick={()=>change("city",c.city)}><span className="stats-rank">{String(i+1).padStart(2,"0")}</span><div><strong>{c.city}</strong><small>{areaNames[c.area]||c.area} · {c.stores} 家门店有记录</small>{c.products?.map(p=><span className="stats-city-product" key={p.model+p.color+p.capacity}><strong>{p.model} · {p.color} · {p.capacity}</strong><small>{p.count} 条 · {p.stores.join("、")} · 最近 {new Date(p.last_at).toLocaleTimeString("zh-CN",{timeZone:"Asia/Shanghai",hour12:false})}</small></span>)}<div className="stats-city-bar" style={{width:`${c.count/data.cities[0].count*100}%`}}/></div><div><b>{c.count} 条</b><small>最近 {new Date(c.last_at).toLocaleTimeString("zh-CN",{timeZone:"Asia/Shanghai",hour12:false})}</small></div></button>):<p className="stats-empty">暂无记录，可切换日期或筛选条件查看。</p>}
   <p className="stats-footnote">仅统计已上报的检测记录，同一分钟同地区、门店、SKU 去重。记录数不等于放货批次或库存数量，也不代表全国全部门店；历史记录不代表实时有货。</p><p className="stats-footnote">每 5 分钟自动刷新 · 最近数据：{data.latest_at?new Date(data.latest_at).toLocaleString("zh-CN",{timeZone:"Asia/Shanghai",hour12:false}):"暂无"}</p>
  </>}
 </section></div>
}
