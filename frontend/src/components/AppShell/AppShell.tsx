import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import Toaster from '../Toaster/Toaster'
import NotificationBadge from '../NotificationBadge/NotificationBadge'
import styles from './AppShell.module.css'

export default function AppShell() {
  const auth = useAuth()
  const navigate = useNavigate()

  const userLabel = auth.isLoggedIn
    ? `${auth.claims?.username ?? '?'} #${auth.claims?.account_id ?? ''}`
    : '未登录'

  return (
    <div className={styles.shell}>
      <aside className={styles.aside}>
        <NavLink className={styles.logo} to="/">ShortVideo</NavLink>

        <nav className={styles.nav}>
          <NavLink className={styles.navLink} to="/" end>推荐</NavLink>
          <NavLink className={styles.navLink} to="/hot">热榜</NavLink>
          <NavLink className={styles.navLink} to="/upload">发布</NavLink>
          <NavLink className={styles.navLink} to="/account">账号</NavLink>
          {auth.isLoggedIn && <NavLink className={styles.navLink} to="/messages">私信</NavLink>}
          {auth.isLoggedIn && <NavLink className={styles.navLink} to="/settings">设置</NavLink>}
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
