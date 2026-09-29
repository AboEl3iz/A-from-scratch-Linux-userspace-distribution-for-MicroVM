import { createContext, useCallback, useContext, useEffect, useState } from 'react'

const ThemeContext = createContext({ theme: 'dark', toggle: () => {} })

function initialTheme() {
  try {
    return localStorage.getItem('karim-theme') === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

export function ThemeProvider({ children }) {
  const [theme, setTheme] = useState(initialTheme)
  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    try {
      localStorage.setItem('karim-theme', theme)
    } catch {
      /* private mode: theme still applies for this session */
    }
  }, [theme])
  const toggle = useCallback(() => setTheme((t) => (t === 'dark' ? 'light' : 'dark')), [])
  return <ThemeContext.Provider value={{ theme, toggle }}>{children}</ThemeContext.Provider>
}

export const useTheme = () => useContext(ThemeContext)
