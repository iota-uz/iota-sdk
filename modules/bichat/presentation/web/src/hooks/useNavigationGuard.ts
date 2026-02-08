import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { isInScopePath } from '../utils/navigation'

export function useNavigationGuard() {
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    const pathname = location.pathname || '/'
    if (isInScopePath(pathname)) {
      return
    }
    navigate('/', { replace: true })
  }, [location.pathname, navigate])
}

