import { useTheme } from '../context/ThemeContext'

export default function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="theme-switch" role="group" aria-label="Tema">
      <button
        type="button"
        aria-pressed={theme === 'light'}
        onClick={() => setTheme('light')}
      >
        Dia
      </button>
      <button
        type="button"
        aria-pressed={theme === 'dark'}
        onClick={() => setTheme('dark')}
      >
        Noite
      </button>
    </div>
  )
}
