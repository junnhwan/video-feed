import { useToast } from '../../stores/toast'
import styles from './Toaster.module.css'

export default function Toaster() {
  const { toasts, remove } = useToast()

  return (
    <div className={styles.wrap} aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`${styles.toast} ${styles[t.type]}`}>
          <div className={styles.msg}>{t.message}</div>
          <button className={styles.close} onClick={() => remove(t.id)} aria-label="关闭">×</button>
        </div>
      ))}
    </div>
  )
}
