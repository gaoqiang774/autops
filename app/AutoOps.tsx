"use client";
import { FormEvent, useEffect, useState } from "react";
import Workbench from "./Workbench";
import CmdbModule from "./cmdb/CmdbModule";
import { modules } from "./data";
import { APP_VERSION } from "./version";
import { UserAccount, initialUsers } from "./cmdb/userTypes";

function Login({ users, onLogin }: { users: UserAccount[]; onLogin: (account: string) => void }) {
  const [account, setAccount] = useState("admin");
  const [password, setPassword] = useState("123456");
  const [error, setError] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    const r = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ account, password }),
    });
    if (r.ok) onLogin(account);
    else setError("账号或密码错误 (默认密码为 123456)");
  }

  return (
    <main className="login-page">
      <div className="orb a" />
      <div className="orb b" />
      <section className="login-card" style={{ maxWidth: 440 }}>
        <img
          src="/lookdoordog.jpg"
          alt="VST LOOK DOOR DOG"
          style={{ height: 68, maxWidth: 220, objectFit: "contain", margin: "0 auto 12px", display: "block", borderRadius: 6 }}
        />
        <h1>运维信息资产管理平台</h1>
        <p>信息资产台账 · 智能配置管理 · 权限精细隔离</p>
        <form onSubmit={submit}>
          <label>账号</label>
          <div className="input-wrap">
            <span>♙</span>
            <input
              aria-label="账号"
              placeholder="请输入账号 (如 admin, ops_beijing)"
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
              placeholder="请输入密码 (默认 123456)"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          {error && <small className="error">{error}</small>}

          {/* Quick preset accounts helper */}
          <div style={{
            background: "rgba(255,255,255,0.06)",
            border: "1px dashed rgba(255,255,255,0.2)",
            borderRadius: 6,
            padding: "8px 10px",
            margin: "12px 0 16px",
            fontSize: 11,
            color: "rgba(255,255,255,0.7)"
          }}>
            <div style={{ marginBottom: 4, fontWeight: 600 }}>快捷账号选择 (测试不同项目授权):</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              {users.map((u) => (
                <button
                  key={u.id}
                  type="button"
                  onClick={() => {
                    setAccount(u.username);
                    setPassword("123456");
                    setError("");
                  }}
                  style={{
                    padding: "3px 7px",
                    background: account === u.username ? "rgba(37,99,235,0.8)" : "rgba(255,255,255,0.12)",
                    border: "1px solid rgba(255,255,255,0.2)",
                    borderRadius: 4,
                    color: "#fff",
                    fontSize: 10,
                    cursor: "pointer"
                  }}
                >
                  {u.username} ({u.authorizedProjects === "all" ? "全量24项目" : u.authorizedProjects.length + "项目"})
                </button>
              ))}
            </div>
          </div>

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
  const [moduleIndex] = useState(0);
  const [page, setPage] = useState("项目资产");
  const [profile, setProfile] = useState(false);
  const [mainSidebarCollapsed, setMainSidebarCollapsed] = useState(false);
  // 欢迎页状态：始终显示，直到用户交互才消失
  const [showSplash, setShowSplash] = useState(true);

  // Users & Permissions state
  const [users, setUsers] = useState<UserAccount[]>(() => {
    if (typeof window !== "undefined") {
      try {
        const saved = localStorage.getItem("autoops_users_v1");
        if (saved) return JSON.parse(saved);
      } catch (e) {}
    }
    return initialUsers;
  });

  const [currentUser, setCurrentUser] = useState<UserAccount>(() => {
    if (typeof window !== "undefined") {
      try {
        const saved = localStorage.getItem("autoops_active_user_v1");
        if (saved) return JSON.parse(saved);
      } catch (e) {}
    }
    return initialUsers[0];
  });

  function handleAddUser(newUser: UserAccount) {
    setUsers((prev) => {
      const updated = [...prev, newUser];
      try { localStorage.setItem("autoops_users_v1", JSON.stringify(updated)); } catch (e) {}
      return updated;
    });
  }

  function handleUpdateUser(updatedUser: UserAccount) {
    setUsers((prev) => {
      const updated = prev.map((u) => (u.id === updatedUser.id ? updatedUser : u));
      try { localStorage.setItem("autoops_users_v1", JSON.stringify(updated)); } catch (e) {}
      return updated;
    });
    if (currentUser.id === updatedUser.id) {
      setCurrentUser(updatedUser);
      try { localStorage.setItem("autoops_active_user_v1", JSON.stringify(updatedUser)); } catch (e) {}
    }
  }

  function handleDeleteUser(userId: string) {
    setUsers((prev) => {
      const updated = prev.filter((u) => u.id !== userId);
      try { localStorage.setItem("autoops_users_v1", JSON.stringify(updated)); } catch (e) {}
      return updated;
    });
  }

  function handleSwitchUser(targetUser: UserAccount) {
    setCurrentUser(targetUser);
    try { localStorage.setItem("autoops_active_user_v1", JSON.stringify(targetUser)); } catch (e) {}
  }

  useEffect(() => {
    // 后台静默检查 session
    fetch("/api/auth/session")
      .then(async (r) => {
        if (r.ok) {
          const data = await r.json();
          setAuthed(true);
          if (data.user && data.user.account) {
            const matched = users.find((u) => u.username === data.user.account);
            if (matched) setCurrentUser(matched);
          }
        } else {
          setAuthed(false);
        }
      })
      .catch(() => setAuthed(false));

    // 监听点击 / 键盘事件，触发后关闭欢迎页
    function dismiss() {
      setShowSplash(false);
    }
    window.addEventListener("click", dismiss, { once: true });
    window.addEventListener("keydown", dismiss, { once: true });
    return () => {
      window.removeEventListener("click", dismiss);
      window.removeEventListener("keydown", dismiss);
    };
  }, []);

  /* ── 全屏欢迎页（等待用户交互） ─────────────────────── */
  if (showSplash) {
    return (
      <div style={{
        position: "fixed", inset: 0,
        display: "flex", flexDirection: "column",
        alignItems: "center", justifyContent: "center",
        background: "linear-gradient(135deg, #0f172a 0%, #1e3a5f 55%, #0f172a 100%)",
        overflow: "hidden"
      }}>
        {/* 背景光晕 */}
        <div style={{
          position: "absolute", inset: 0,
          background: "radial-gradient(ellipse 70% 50% at 50% 48%, rgba(37,99,235,0.22) 0%, transparent 68%)",
          pointerEvents: "none"
        }} />
        {/* Logo 大图 */}
        <img
          src="/lookdoordog.jpg"
          alt="VST LOOK DOOR DOG"
          style={{
            position: "relative", zIndex: 1,
            width: "min(520px, 74vw)",
            height: "auto",
            objectFit: "contain",
            borderRadius: 18,
            boxShadow: "0 12px 60px rgba(37,99,235,0.4), 0 0 0 1px rgba(99,179,237,0.15)"
          }}
        />
        {/* 标题 */}
        <div style={{ position: "relative", zIndex: 1, textAlign: "center", marginTop: 40 }}>
          <div style={{ fontSize: 28, fontWeight: 700, color: "#fff", letterSpacing: 3, textShadow: "0 2px 20px rgba(37,99,235,0.6)" }}>
            运维信息资产管理平台
          </div>
          <div style={{ fontSize: 13, color: "rgba(255,255,255,0.45)", marginTop: 10, letterSpacing: 1.5 }}>
            信息资产台账 · 智能配置管理 · 漏洞精准排查
          </div>
          {/* 点击提示 */}
          <div style={{ marginTop: 40, display: "flex", flexDirection: "column", alignItems: "center", gap: 12 }}>
            <div style={{
              fontSize: 13,
              color: "rgba(255,255,255,0.5)",
              letterSpacing: 2,
              animation: "splashBlink 2s ease-in-out infinite"
            }}>
              点击任意处 · 按任意键进入系统
            </div>
            <div style={{ display: "flex", gap: 6 }}>
              {[0, 1, 2, 3, 4].map(i => (
                <div key={i} style={{
                  width: 5, height: 5, borderRadius: "50%",
                  background: `rgba(99,179,237,${0.3 + i * 0.14})`,
                }} />
              ))}
            </div>
          </div>
        </div>
        <style>{`
          @keyframes bp{0%,100%{opacity:.25;transform:scale(.75)}50%{opacity:1;transform:scale(1.25)}}
          @keyframes splashBlink{0%,100%{opacity:0.3}50%{opacity:1}}
        `}</style>
      </div>
    );
  }

  if (!authed) {
    return (
      <Login
        users={users}
        onLogin={(acc) => {
          setAuthed(true);
          const matched = users.find((u) => u.username === acc);
          if (matched) {
            handleSwitchUser(matched);
          }
        }}
      />
    );
  }

  const mod = modules[moduleIndex];

  return (
    <div className="shell">
      <header className="topbar">
        {/* 顶部左侧标识 */}
        <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 160 }}>
          <div style={{
            width: 32,
            height: 32,
            borderRadius: 8,
            background: "linear-gradient(135deg, #2563eb, #1d4ed8)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "#fff",
            fontSize: 15,
            boxShadow: "0 2px 8px rgba(37,99,235,0.22)"
          }}>
            ❖
          </div>
          <div style={{ display: "flex", flexDirection: "column", lineHeight: 1.15 }}>
            <span style={{ fontSize: 13, fontWeight: 700, color: "#1e293b", letterSpacing: "1px" }}>AutoOps</span>
            <span style={{ fontSize: 9, color: "#64748b", letterSpacing: "0.5px", fontWeight: 600 }}>SECURE OPS</span>
          </div>
        </div>

        {/* 系统主标题 - 居中放大、字体美化、字间距放大 */}
        <div style={{
          position: "absolute",
          left: "50%",
          transform: "translateX(-50%)",
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 14,
          padding: "8px 38px",
          background: "linear-gradient(135deg, rgba(255,255,255,0.95), rgba(239,246,255,0.9))",
          border: "1.5px solid rgba(147,197,253,0.7)",
          borderRadius: 28,
          boxShadow: "0 4px 18px rgba(37,99,235,0.1), 0 1px 2px rgba(0,0,0,0.04), inset 0 1px 0 rgba(255,255,255,0.9)",
          whiteSpace: "nowrap"
        }}>
          <span style={{
            display: "inline-block",
            width: 6,
            height: 6,
            borderRadius: "50%",
            background: "linear-gradient(135deg, #3b82f6, #1d4ed8)",
            boxShadow: "0 0 8px #3b82f6"
          }} />
          <span style={{
            fontSize: 22,
            fontWeight: 800,
            letterSpacing: 10,
            paddingLeft: 10,
            background: "linear-gradient(135deg, #09203f 0%, #1e3a8a 52%, #1d4ed8 100%)",
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
            filter: "drop-shadow(0 1px 2px rgba(30,58,138,0.15))",
            display: "inline-block"
          }}>
            运维信息管理
          </span>
          <span style={{
            display: "inline-block",
            width: 6,
            height: 6,
            borderRadius: "50%",
            background: "linear-gradient(135deg, #3b82f6, #1d4ed8)",
            boxShadow: "0 0 8px #3b82f6"
          }} />
        </div>

        <div className="account">
          <div
            className="version-pill"
            title={`运维信息资产管理平台 v${APP_VERSION}\n版本规则：修复Bug加末位，增加模块加中位，增加主菜单改首位`}
          >
            <span className="version-dot" />
            <span style={{ fontWeight: 700, letterSpacing: "0.5px" }}>v{APP_VERSION}</span>
          </div>

          <i>🐱</i>
          <button
            className="account-btn"
            onClick={() => setProfile(!profile)}
            style={{ display: "inline-flex", alignItems: "center", gap: 6 }}
          >
            <span>{currentUser.displayName}</span>
            <span style={{
              fontSize: 10,
              padding: "1px 5px",
              borderRadius: 3,
              background: currentUser.role === "admin" ? "#eff6ff" : "#f0fdf4",
              color: currentUser.role === "admin" ? "#1d4ed8" : "#15803d",
              fontWeight: 700,
              border: currentUser.role === "admin" ? "1px solid #bfdbfe" : "1px solid #bbf7d0"
            }}>
              {currentUser.role === "admin"
                ? "Admin"
                : `${currentUser.authorizedProjects === "all" ? "全量" : currentUser.authorizedProjects.length + "项目"}`}
            </span>
            <span style={{ fontSize: 10 }}>⌄</span>
          </button>

          {profile && (
            <div className="profile-menu" style={{ width: 280, padding: 12 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8, borderBottom: "1px solid #e2e8f0", paddingBottom: 8 }}>
                <div style={{
                  width: 32,
                  height: 32,
                  borderRadius: "50%",
                  background: currentUser.role === "admin" ? "#eff6ff" : "#f0fdf4",
                  color: currentUser.role === "admin" ? "#1d4ed8" : "#15803d",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontWeight: 700
                }}>
                  {currentUser.displayName.slice(0, 1)}
                </div>
                <div>
                  <b style={{ display: "block", fontSize: 13 }}>{currentUser.displayName}</b>
                  <span style={{ fontSize: 11, color: "#64748b" }}>@{currentUser.username} · {currentUser.department}</span>
                </div>
              </div>

              <div style={{ fontSize: 11, color: "#334155", marginBottom: 10, background: "#f8fafc", padding: "6px 8px", borderRadius: 4 }}>
                <strong>项目权限范围:</strong>{" "}
                <span style={{ color: currentUser.role === "admin" ? "#1d4ed8" : "#15803d", fontWeight: 600 }}>
                  {currentUser.authorizedProjects === "all" ? "全网 24 个项目 (全局可见)" : `已授权 ${currentUser.authorizedProjects.length} 个项目`}
                </span>
              </div>

              {/* Quick switch user identity menu */}
              <div style={{ marginBottom: 12 }}>
                <span style={{ fontSize: 10, color: "#94a3b8", display: "block", marginBottom: 5 }}>
                  ⚡ 快速切换身份视角 (测试项目权限隔离):
                </span>
                <div style={{ display: "flex", flexDirection: "column", gap: 4, maxHeight: 160, overflowY: "auto" }}>
                  {users.map((u) => {
                    const active = u.id === currentUser.id;
                    return (
                      <button
                        key={u.id}
                        type="button"
                        onClick={() => {
                          handleSwitchUser(u);
                          setProfile(false);
                        }}
                        style={{
                          padding: "4px 8px",
                          fontSize: 11,
                          borderRadius: 4,
                          border: active ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                          background: active ? "#eff6ff" : "#fff",
                          color: active ? "#1d4ed8" : "#334155",
                          textAlign: "left",
                          cursor: "pointer",
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center"
                        }}
                      >
                        <span style={{ fontWeight: active ? 700 : 400 }}>{u.displayName}</span>
                        <small style={{ color: active ? "#1d4ed8" : "#64748b" }}>
                          {u.authorizedProjects === "all" ? "全量24项目" : `${u.authorizedProjects.length}个项目`}
                        </small>
                      </button>
                    );
                  })}
                </div>
              </div>

              <button
                style={{
                  width: "100%",
                  padding: "6px 0",
                  fontSize: 12,
                  color: "#dc2626",
                  background: "#fef2f2",
                  border: "1px solid #fee2e2",
                  borderRadius: 6,
                  cursor: "pointer",
                  fontWeight: 600
                }}
                onClick={async () => {
                  await fetch("/api/auth/logout", { method: "POST" });
                  setAuthed(false);
                  setProfile(false);
                }}
              >
                退出系统登录
              </button>
            </div>
          )}
        </div>
      </header>

      <aside
        className="sidebar"
        style={{
          width: mainSidebarCollapsed ? 56 : 200,
          padding: mainSidebarCollapsed ? "10px 4px 110px" : "10px 8px 160px",
          transition: "width 0.25s ease, padding 0.25s ease",
          overflow: "hidden",
          boxShadow: "1px 0 4px rgba(0,0,0,0.04)"
        }}
      >
        {/* Top Header Row with prominent collapse button */}
        <div style={{
          display: "flex",
          alignItems: "center",
          justifyContent: mainSidebarCollapsed ? "center" : "space-between",
          padding: mainSidebarCollapsed ? "0 2px 10px" : "4px 8px 10px",
          borderBottom: "1px solid #e2e8f0",
          marginBottom: 10,
          overflow: "hidden",
          whiteSpace: "nowrap"
        }}>
          {!mainSidebarCollapsed ? (
            <>
              <span style={{ fontSize: 13, color: "#64748b", fontWeight: 700, display: "flex", alignItems: "center", gap: 6 }}>
                <span>{mod.icon}</span>
                <span>{mod.name}</span>
              </span>
              <button
                type="button"
                onClick={() => setMainSidebarCollapsed(true)}
                title="收缩主导航侧栏"
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: 4,
                  border: "1px solid #cbd5e1",
                  background: "#fff",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 13,
                  fontWeight: 700,
                  color: "#475569",
                  padding: 0
                }}
              >
                ‹
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={() => setMainSidebarCollapsed(false)}
              title="展开主导航侧栏"
              style={{
                width: 28,
                height: 28,
                borderRadius: 6,
                border: "1.5px solid #93c5fd",
                background: "#eff6ff",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: 14,
                fontWeight: 700,
                color: "#2563eb",
                padding: 0
              }}
            >
              ›
            </button>
          )}
        </div>

        {mod.menus.map((x, i) => (
          <button
            key={x}
            className={page === x ? "active" : ""}
            onClick={() => setPage(x)}
            title={mainSidebarCollapsed ? x : undefined}
            style={{
              padding: mainSidebarCollapsed ? "0" : "0 18px",
              display: "flex",
              alignItems: "center",
              justifyContent: mainSidebarCollapsed ? "center" : "flex-start",
              width: "100%",
              height: 40,
              gap: mainSidebarCollapsed ? 0 : 8,
              fontSize: 14,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              marginBottom: 4,
              borderRadius: 8
            }}
          >
            <span style={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              width: mainSidebarCollapsed ? "auto" : 24,
              fontSize: 16,
              flexShrink: 0
            }}>
              {x === "项目资产" ? "▣" :
               x === "资产大盘" ? "♙" :
               x === "业务拓扑" ? "▰" :
               x === "漏洞检测" ? "♧" :
               x === "凭据管理" ? "▥" :
               x === "用户管理" ? "👥" :
               x === "AIops助手" ? "◉" : "▢"}
            </span>
            {!mainSidebarCollapsed && <span>{x}</span>}
          </button>
        ))}

        {/* 收起/展开主导航按钮 */}
        <button
          className="collapse"
          type="button"
          onClick={() => setMainSidebarCollapsed(!mainSidebarCollapsed)}
          title={mainSidebarCollapsed ? "展开主导航" : "收缩主导航"}
          style={{
            position: "absolute",
            bottom: mainSidebarCollapsed ? 58 : 110,
            left: mainSidebarCollapsed ? 8 : 40,
            width: mainSidebarCollapsed ? 40 : 120,
            height: 32,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 6,
            background: "#fff",
            border: "1px solid #cbd5e1",
            borderRadius: 6,
            color: "#475569",
            fontSize: 13,
            fontWeight: 600,
            cursor: "pointer",
            boxShadow: "0 1px 3px rgba(0,0,0,0.05)",
            transition: "all 0.2s ease",
            zIndex: 2
          }}
        >
          <span>{mainSidebarCollapsed ? "›" : "‹"}</span>
          {!mainSidebarCollapsed && <span>收起导航</span>}
        </button>

        {/* 左下角品牌图标套组 - 底色与第一栏背景融为一体 */}
        <div
          title="VST LOOK DOOR DOG - SECURITY SOLUTIONS"
          style={{
            position: "absolute",
            bottom: 8,
            left: 0,
            right: 0,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            padding: mainSidebarCollapsed ? "2px" : "4px 10px",
            background: "transparent",
            transition: "all 0.25s ease",
            overflow: "hidden",
            zIndex: 1
          }}
        >
          <img
            src="/lookdoordog.jpg"
            alt="VST LOOK DOOR DOG"
            style={{
              width: mainSidebarCollapsed ? 44 : 176,
              height: mainSidebarCollapsed ? 44 : "auto",
              maxHeight: mainSidebarCollapsed ? 44 : 94,
              objectFit: mainSidebarCollapsed ? "cover" : "contain",
              objectPosition: mainSidebarCollapsed ? "center 15%" : "center",
              borderRadius: mainSidebarCollapsed ? 8 : 8,
              display: "block",
              mixBlendMode: "multiply",
              filter: "contrast(1.06) brightness(1.01)",
              transition: "all 0.25s ease"
            }}
          />
        </div>
      </aside>

      {moduleIndex === 0 ? (
        <CmdbModule
          page={page}
          onPageChange={(p) => setPage(p)}
          mainSidebarCollapsed={mainSidebarCollapsed}
          currentUser={currentUser}
          users={users}
          onAddUser={handleAddUser}
          onUpdateUser={handleUpdateUser}
          onDeleteUser={handleDeleteUser}
          onSwitchUser={handleSwitchUser}
        />
      ) : (
        <Workbench moduleName={mod.name} page={page} />
      )}
    </div>
  );
}
