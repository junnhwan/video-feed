import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import Toaster from '../Toaster/Toaster'
import NotificationBadge from '../NotificationBadge/NotificationBadge'
import styles from './AppShell.module.css'

/* ── Inline SVG icons ── */
const icons = {
  home: (
    <svg viewBox="0 0 24 24">
      <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
    </svg>
  ),
  hot: (
    <svg viewBox="0 0 24 24">
      <path d="M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z" />
      <path d="M9.879 16.121A3 3 0 1012.015 11L11 14H9c0 .768.293 1.536.879 2.121z" />
    </svg>
  ),
  upload: (
    <svg viewBox="0 0 24 24">
      <path d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
    </svg>
  ),
  user: (
    <svg viewBox="0 0 24 24">
      <path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
    </svg>
  ),
  message: (
    <svg viewBox="0 0 24 24">
      <path d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 24 24">
      <path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ),
}

export default function AppShell() {
  const auth = useAuth()
  const navigate = useNavigate()

  const userLabel = auth.isLoggedIn
    ? `${auth.claims?.username ?? '?'} #${auth.claims?.account_id ?? ''}`
    : '未登录'

  return (
    <div className={styles.shell}>
      <aside className={styles.aside}>
        <NavLink className={styles.logo} to="/">
          <span className={styles.logoIcon}>
            <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z" fill="#fff" stroke="none"/></svg>
          </span>
          <span className={styles.logoText}>ShortVideo</span>
        </NavLink>

        <nav className={styles.nav}>
          <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/" end>
            <span className={styles.navIcon}>{icons.home}</span>推荐
          </NavLink>
          <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/hot">
            <span className={styles.navIcon}>{icons.hot}</span>热榜
          </NavLink>
          <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/upload">
            <span className={styles.navIcon}>{icons.upload}</span>发布
          </NavLink>
          <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/account">
            <span className={styles.navIcon}>{icons.user}</span>账号
          </NavLink>
          {auth.isLoggedIn && (
            <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/messages">
              <span className={styles.navIcon}>{icons.message}</span>私信
            </NavLink>
          )}
          {auth.isLoggedIn && (
            <NavLink className={({ isActive }) => `${styles.navLink} ${isActive ? styles.activeIndicator : ''}`} to="/settings">
              <span className={styles.navIcon}>{icons.settings}</span>设置
            </NavLink>
          )}
        </nav>

        <div className={styles.asideFoot}>
          {auth.isLoggedIn && (
            <div className={styles.notifRow}>
              <NotificationBadge />
            </div>
          )}
          <div className={styles.user}>
            <span className={`${styles.dot} ${auth.isLoggedIn ? styles.ok : styles.bad}`} />
            <span className={styles.userName}>{userLabel}</span>
          </div>
          <button
            className={`${styles.btn} ${styles.primary}`}
            onClick={() => navigate(auth.isLoggedIn ? '/settings' : '/account')}
          >
            {auth.isLoggedIn ? '设置' : '登录'}
          </button>
        </div>
      </aside>

      <div className={styles.main}>
        <nav className={styles.mobileNav}>
          <NavLink className={styles.mobileLink} to="/" end>推荐</NavLink>
          <NavLink className={styles.mobileLink} to="/hot">热榜</NavLink>
          <NavLink className={styles.mobileLink} to="/upload">发布</NavLink>
          {auth.isLoggedIn && <NavLink className={styles.mobileLink} to="/messages">私信</NavLink>}
          <NavLink className={styles.mobileLink} to="/account">账号</NavLink>
        </nav>

        <div className={styles.content}>
          <Outlet />
        </div>
      </div>

      <Toaster />
    </div>
  )
}
