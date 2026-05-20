import { useState, type FormEvent } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import * as accountApi from '../../api/account'
import styles from './ChangePasswordView.module.css'

export default function ChangePasswordView() {
  const auth = useAuth()
  const toast = useToast()
  const navigate = useNavigate()

  const [username, setUsername] = useState(auth.claims?.username ?? '')
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!username.trim() || !oldPassword || !newPassword) return

    setLoading(true)
    try {
      await accountApi.changePassword(username.trim(), oldPassword, newPassword)
      toast.success('密码已修改，请重新登录')
      auth.clearTokens()
      navigate('/account')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '修改密码失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.card}>
        <h2 className={styles.title}>修改密码</h2>

        <form className={styles.form} onSubmit={handleSubmit}>
          <label className={styles.label}>
            用户名
            <input
              className={styles.input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
            />
          </label>

          <label className={styles.label}>
            当前密码
            <input
              className={styles.input}
              type="password"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              autoComplete="current-password"
            />
          </label>

          <label className={styles.label}>
            新密码
            <input
              className={styles.input}
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              autoComplete="new-password"
            />
          </label>

          <button className={`${styles.btn} ${styles.primary}`} type="submit" disabled={loading}>
            {loading ? '提交中...' : '确认修改'}
          </button>
        </form>

        <p className={styles.back}>
          <Link className={styles.link} to="/settings">
            返回设置
          </Link>
        </p>
      </div>
    </div>
  )
}
