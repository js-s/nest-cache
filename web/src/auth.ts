import { ref } from 'vue'
import { logout as apiLogout, me, type Profile } from './api/client'

// Minimal session state: undefined = unchecked (boot), null = guest.
const user = ref<Profile | null | undefined>(undefined)

export function useSession() {
  return { user }
}

export async function ensureSession(): Promise<Profile | null> {
  if (user.value !== undefined) {
    return user.value
  }
  try {
    user.value = await me()
  } catch {
    user.value = null
  }
  return user.value
}

export function setSession(profile: Profile | null): void {
  user.value = profile
}

export async function logoutSession(): Promise<void> {
  try {
    await apiLogout()
  } finally {
    user.value = null
  }
}
