import { ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'nestcash-theme'

function systemQuery(): MediaQueryList | null {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(prefers-color-scheme: dark)')
    : null
}

function systemTheme(): Theme {
  return systemQuery()?.matches === true ? 'dark' : 'light'
}

function effectiveTheme(): Theme {
  const applied = document.documentElement.dataset.theme
  if (applied === 'light' || applied === 'dark') {
    return applied
  }
  return systemTheme()
}

export const theme = ref<Theme>(effectiveTheme())

export function toggleTheme(): void {
  const next: Theme = effectiveTheme() === 'dark' ? 'light' : 'dark'
  document.documentElement.dataset.theme = next
  try {
    localStorage.setItem(STORAGE_KEY, next)
  } catch {
    /* ignore */
  }
  theme.value = next
}

const systemQueryList = systemQuery()
if (systemQueryList) {
  const onSystemChange = (): void => {
    if (!document.documentElement.dataset.theme) {
      theme.value = systemTheme()
    }
  }
  if (typeof systemQueryList.addEventListener === 'function') {
    systemQueryList.addEventListener('change', onSystemChange)
  } else {
    systemQueryList.addListener(onSystemChange)
  }
}
