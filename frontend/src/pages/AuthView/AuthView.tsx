import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../../stores/auth'
import { useToast } from '../../stores/toast'
import * as accountApi from '../../api/account'
import styles from './AuthView.module.css'

export default function AuthView() {
  const [isRegister, setIsRegister] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const auth = useAuth()
  const toast = useToast()
  const navigate = useNavigate()
  const [params] = useSearchParams()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!username.trim() || !password.trim()) return

    setLoading(true)
    try {
      if (isRegister) {
        await accountApi.register(username.trim(), password)
        toast.success('注册成功，请登录')
        setIsRegister(false)
        setPassword('')
      } else {
        const res = await accountApi.login(username.trim(), password)
        auth.setTokens(res.token, res.refresh_token ?? '')
        toast.success(`欢迎回来，${res.username}`)
        const redirect = params.get('redirect') ?? '/'
        navigate(redirect)
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '操作失败'
      toast.error(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.card}>
        {/* Brand header */}
        <div className={styles.brand}>
          <div className={styles.logoIcon}>
            <svg viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
          </div>
          <div className={styles.brandName}>ShortVideo</div>
          <div className={styles.brandSlogan}>发现精彩，记录生活</div>
        </div>

        {/* Login / Register toggle */}
        <div className={styles.toggle}>
          <div className={`${styles.toggleSlider} ${isRegister ? styles.toggleSliderRight : ''}`} />
          <button
            className={`${styles.toggleBtn} ${!isRegister ? styles.toggleBtnActive : ''}`}
            onClick={() => setIsRegister(false)}
          >登录</button>
          <button
            className={`${styles.toggleBtn} ${isRegister ? styles.toggleBtnActive : ''}`}
            onClick={() => setIsRegister(true)}
          >注册</button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit} key={isRegister ? 'reg' : 'login'}>
          <label className={styles.label}>
            用户名
            <input
              className={styles.input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入用户名"
              autoComplete="username"
              autoFocus
            />
          </label>

          <label className={styles.label}>
            密码
            <input
              className={styles.input}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="请输入密码"
              autoComplete={isRegister ? 'new-password' : 'current-password'}
            />
          </label>

          <button className={`${styles.btn} ${styles.primary}`} type="submit" disabled={loading}>
            {loading ? '处理中...' : (isRegister ? '注册' : '登录')}
          </button>
        </form>
      </div>
    </div>
  )
}
