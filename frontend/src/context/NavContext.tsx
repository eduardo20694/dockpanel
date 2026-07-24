import { createContext, useContext, useEffect, useState, ReactNode, useCallback } from 'react'
import { useLocation } from 'react-router-dom'

interface NavContextValue {
  open: boolean
  setOpen: (v: boolean) => void
  toggle: () => void
  close: () => void
}

const NavContext = createContext<NavContextValue>({
  open: false,
  setOpen: () => {},
  toggle: () => {},
  close: () => {},
})

export function NavProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const location = useLocation()

  const close = useCallback(() => setOpen(false), [])
  const toggle = useCallback(() => setOpen((v) => !v), [])

  useEffect(() => {
    setOpen(false)
  }, [location.pathname])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  }, [open])

  return (
    <NavContext.Provider value={{ open, setOpen, toggle, close }}>
      {children}
    </NavContext.Provider>
  )
}

export function useNav() {
  return useContext(NavContext)
}
