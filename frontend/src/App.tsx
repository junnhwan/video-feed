import { RouterProvider } from 'react-router-dom'
import { AuthProvider } from './stores/auth'
import { ToastProvider } from './stores/toast'
import { SocialProvider } from './stores/social'
import { router } from './router'

export default function App() {
  return (
    <AuthProvider>
      <ToastProvider>
        <SocialProvider>
          <RouterProvider router={router} />
        </SocialProvider>
      </ToastProvider>
    </AuthProvider>
  )
}
