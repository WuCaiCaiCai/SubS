import React from 'react'
import { createRoot } from 'react-dom/client'
import { Activity, Boxes, CircleHelp, KeyRound, Layers3, LockKeyhole, Plane, Server, Settings2, ShieldCheck } from 'lucide-react'
import './style.css'

function App() {
  return <main className="shell">
    <aside className="sidebar"><div className="brand"><span className="brand-mark">S</span><div><b>SubS</b><small>PRIVATE SUBSCRIPTION OPS</small></div></div><nav><a className="active"><Activity size={17}/>总览</a><a><Layers3 size={17}/>来源管理</a><a><Boxes size={17}/>节点库</a><a><Settings2 size={17}/>订阅编排</a></nav><div className="side-bottom"><a><CircleHelp size={17}/>帮助与状态</a><div className="secure"><ShieldCheck size={15}/><span>本地运行 · 私有实例</span></div></div></aside>
    <section className="content"><header><div><div className="eyebrow">WORKSPACE / OVERVIEW</div><h1>运行概览</h1></div><button className="avatar" aria-label="账户设置">W</button></header>
      <div className="status-line"><span className="dot"/>所有服务正常 <span className="divider"/>实例 <code>my-subs</code><span className="divider"/>版本 <code>dev</code></div>
      <div className="section-head"><h2>来源</h2><button className="quiet">管理来源 <span>→</span></button></div>
      <div className="source-grid"><article className="source-card airport"><div className="card-top"><div className="source-icon"><Plane size={18}/></div><span className="overline">AIRPORTS</span><button className="more">···</button></div><strong>0</strong><div className="label">机场订阅</div><div className="card-bottom"><span>尚未添加来源</span><button className="text-button">添加机场 <span>＋</span></button></div></article>
      <article className="source-card server"><div className="card-top"><div className="source-icon"><Server size={18}/></div><span className="overline">SELF-HOSTED</span><button className="more">···</button></div><strong>0</strong><div className="label">自建 VPS</div><div className="card-bottom"><span>尚未添加服务器</span><button className="text-button">添加 VPS <span>＋</span></button></div></article></div>
      <div className="section-head profile-head"><div><h2>订阅编排</h2><p>组合节点、应用模板，并生成客户端订阅链接。</p></div><button className="primary"><span>＋</span> 新建编排</button></div>
      <section className="empty-panel"><div className="empty-icon"><KeyRound size={20}/></div><div><b>还没有订阅编排</b><p>从机场或自建节点中选择节点，创建你的第一条订阅。</p></div><button className="outline">开始创建 <span>→</span></button></section>
      <footer><span><LockKeyhole size={13}/> 凭据仅存储在此实例</span><span>数据不会发送到第三方</span></footer>
    </section>
  </main>
}

createRoot(document.getElementById('root')!).render(<React.StrictMode><App/></React.StrictMode>)
