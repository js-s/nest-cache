<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError, login } from '../api/client'
import { setSession } from '../auth'

const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

function valid(): boolean {
  return /.+@.+\..+/.test(email.value.trim()) && password.value.length >= 8
}

async function submit(): Promise<void> {
  error.value = ''
  if (!valid()) {
    error.value = 'Enter a valid email and a password of at least 8 characters.'
    return
  }
  busy.value = true
  try {
    setSession(await login(email.value.trim(), password.value))
    await router.push('/summary')
  } catch (e) {
    // Generic on purpose: no account oracle (bad email vs bad password).
    error.value =
      e instanceof ApiError && e.status === 401
        ? 'Invalid email or password.'
        : 'Login failed. Please try again.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main>
    <h1>Log in</h1>
    <form @submit.prevent="submit">
      <label>Email <input v-model="email" type="email" autocomplete="email" required /></label>
      <label>Password <input v-model="password" type="password" autocomplete="current-password" required minlength="8" /></label>
      <p v-if="error" role="alert">{{ error }}</p>
      <button type="submit" :disabled="busy || !valid()">Log in</button>
    </form>
    <p>No account yet? <router-link to="/register">Register</router-link></p>
    <p>Password reset is coming in v2 — for now, keep your password safe.</p>
  </main>
</template>
