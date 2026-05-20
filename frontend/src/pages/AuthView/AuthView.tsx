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
        <h2 className={styles.title}>{isRegister ? '注册账号' : '登录'}</h2>

        <form className={styles.form} onSubmit={handleSubmit}>
          <label className={styles.label}>
            用户名
            <input
              className={styles.input}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
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
              autoComplete={isRegister ? 'new-password' : 'current-password'}
            />
          </label>

          <button className={`${styles.btn} ${styles.primary}`} type="submit" disabled={loading}>
            {loading ? '处理中...' : (isRegister ? '注册' : '登录')}
          </button>
        </form>

        <p className={styles.switch}>
          {isRegister ? '已有账号？' : '没有账号？'}
          <button className={styles.linkBtn} onClick={() => setIsRegister(!isRegister)}>
            {isRegister ? '去登录' : '去注册'}
          </button>
        </p>
      </div>
    </div>
  )
}
