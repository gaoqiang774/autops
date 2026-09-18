"use client";
import { FormEvent, useEffect, useState } from "react";
import Workbench from "./Workbench";
import CmdbModule from "./cmdb/CmdbModule";
import { modules } from "./data";
import { APP_VERSION } from "./version";

function Login({ onLogin }: { onLogin: () => void }) {
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    const r = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ account, password }),
    });
    if (r.ok) onLogin();
    else setError("账号或密码错误");
  }

  return (
    <main className="login-page">
      <div className="orb a" />
      <div className="orb b" />
      <section className="login-card">
        <img
          src="/lookdoordog.jpg"
          alt="VST LOOK DOOR DOG"
          style={{ height: 68, maxWidth: 220, objectFit: "contain", margin: "0 auto 12px", display: "block", borderRadius: 6 }}
        />
        <h1>运维信息资产管理平台</h1>
        <p>信息资产台账 · 智能配置管理 · 漏洞精准排查</p>
        <form onSubmit={submit}>
          <label>账号</label>
          <div className="input-wrap">
            <span>♙</span>
            <input
              aria-label="账号"
              placeholder="请输入账号"
              value={account}
              onChange={(e) => setAccount(e.target.value)}
            />
          </div>
          <label>密码</label>
          <div className="input-wrap">
            <span>⌾</span>
            <input
              aria-label="密码"
              type="password"
              placeholder="请输入密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          {error && <small className="error">{error}</small>}
          <button className="login-btn">登录系统</button>
          <button
            type="button"
            className="reset-btn"
            onClick={() => {
              setAccount("");
              setPassword("");
              setError("");
            }}
          >
            重置表单
          </button>
        </form>
      </section>
    </main>
  );
}

export default function AutoOps() {
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [moduleIndex, setModuleIndex] = useState(0);
  const [page, setPage] = useState("项目资产");
  const [nav, setNav] = useState(false);
  const [profile, setProfile] = useState(false);

  useEffect(() => {
    fetch("/api/auth/session")
      .then((r) => setAuthed(r.ok))
      .catch(() => setAuthed(false));
  }, []);

  if (authed === null) {
    return (
      <div className="boot" style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12 }}>
        <img
          src="/lookdoordog.jpg"
          alt="VST LOOK DOOR DOG"
          style={{ height: 60, maxWidth: 200, objectFit: "contain", borderRadius: 6 }}
        />
        <span>运维信息资产管理平台</span>
      </div>
    );
  }

  if (!authed) return <Login onLogin={() => setAuthed(true)} />;

  const mod = modules[moduleIndex];

  function chooseModule(i: number) {
    setModuleIndex(i);
    setPage(modules[i].menus[0]);
    setNav(false);
  }

  return (
    <div className="shell">
      <header className="topbar">
        <div className="logo" title="VST LOOK DOOR DOG - 运维信息资产管理" style={{ display: "flex", alignItems: "center", marginRight: 16 }}>
          <img
            src="/lookdoordog.jpg"
            alt="VST LOOK DOOR DOG"
            style={{ height: 46, maxWidth: 170, objectFit: "contain", borderRadius: 4, display: "block" }}
          />
        </div>

        <button className="nav-pill" onClick={() => setNav(!nav)} style={{ display: "inline-flex", alignItems: "center", gap: 4, whiteSpace: "nowrap" }}>
          <span>运维信息资产管理</span>
          <span style={{ fontSize: 11 }}>⌄</span>
        </button>

        <div className="account">
          <div
            className="version-pill"
            title={`运维信息资产管理平台 v${APP_VERSION}\n版本规则：修复Bug加末位，增加模块加中位，增加主菜单改首位`}
          >
            <span className="version-dot" />
            <span style={{ fontWeight: 700, letterSpacing: "0.5px" }}>v{APP_VERSION}</span>
          </div>

          <i>🐱</i>
          <button className="account-btn" onClick={() => setProfile(!profile)}>
            <span>admin</span>
            <span style={{ fontSize: 10 }}>⌄</span>
          </button>

          {profile && (
            <div className="profile-menu">
              <b>管理员</b>
              <span>admin</span>
              <button
                onClick={async () => {
                  await fetch("/api/auth/logout", { method: "POST" });
                  setAuthed(false);
                }}
              >
                退出登录
              </button>
            </div>
          )}
        </div>
      </header>

      <aside className="sidebar">
        <h3>
          {mod.icon}　{mod.name}
        </h3>
        {mod.menus.map((x, i) => (
          <button
            key={x}
            className={page === x ? "active" : ""}
            onClick={() => setPage(x)}
          >
            <span>{["▣", "♙", "▰", "♧", "▥", "◉", "▢", "◔", "◌", "⬡"][i]}</span>
            {x}
          </button>
        ))}
        <button className="collapse">☷</button>
      </aside>

      {moduleIndex === 0 ? (
        <CmdbModule page={page} onPageChange={(p) => setPage(p)} />
      ) : (
        <Workbench moduleName={mod.name} page={page} />
      )}

      {nav && (
        <>
          <button
            aria-label="关闭导航"
            className="nav-backdrop"
            onClick={() => setNav(false)}
          />
          <section className="mega-nav">
            <header>
              <div>
                <b>运维信息资产管理</b>
                <span>快速切换业务模块</span>
              </div>
              <button onClick={() => setNav(false)}>×</button>
            </header>
            <div>
              {modules.map((m, i) => (
                <button
                  key={m.name}
                  className={i === moduleIndex ? "selected" : ""}
                  onClick={() => chooseModule(i)}
                >
                  <i>{m.icon}</i>
                  <span>
                    <b>{m.name}</b>
                    <small>{m.menus.length} 个菜单入口</small>
                  </span>
                  <em>›</em>
                </button>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  );
}
