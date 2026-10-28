import { create } from 'zustand'

function read<T>(key: string, def: T): T {
  try {
    const v = localStorage.getItem(key)
    return v ? (JSON.parse(v) as T) : def
  } catch {
    return def
  }
}
function write(key: string, v: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(v))
  } catch {
    /* ignore */
  }
}

interface Prefs {
  buildingId: string | null
  theme: 'light' | 'dark'
  sidebarOpen: boolean
  setBuilding: (id: string | null) => void
  toggleTheme: () => void
  setSidebar: (open: boolean) => void
}

export const usePrefs = create<Prefs>((set, get) => ({
  buildingId: read('pref.building', null),
  theme: read('pref.theme', 'light'),
  sidebarOpen: false,
  setBuilding: (id) => {
    write('pref.building', id)
    set({ buildingId: id })
  },
  toggleTheme: () => {
    const theme = get().theme === 'dark' ? 'light' : 'dark'
    write('pref.theme', theme)
    try {
      localStorage.setItem('theme', theme)
    } catch {
      /* ignore */
    }
    document.documentElement.classList.toggle('dark', theme === 'dark')
    set({ theme })
  },
  setSidebar: (open) => set({ sidebarOpen: open }),
}))
