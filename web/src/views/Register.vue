<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError, register } from '../api/client'
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
    setSession(await register(email.value.trim(), password.value))
    await router.push('/summary')
  } catch (e) {
    error.value =
      e instanceof ApiError && e.code === 'email_taken'
        ? 'This email is already registered. Try logging in instead.'
        : 'Registration failed. Please try again.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth">
    <div class="card stack">
      <h1>Create account</h1>
      <form class="stack" @submit.prevent="submit">
        <label class="field">
          <span>Email</span>
          <input v-model="email" type="email" autocomplete="email" required />
        </label>
        <label class="field">
          <span>Password (min 8)</span>
          <input v-model="password" type="password" autocomplete="new-password" required minlength="8" />
        </label>
        <p v-if="error" class="alert alert-error" role="alert">{{ error }}</p>
        <button type="submit" class="btn btn-primary" :disabled="busy || !valid()">Register</button>
      </form>
      <p class="muted">Already have an account? <router-link to="/login">Log in</router-link></p>
    </div>
  </div>
</template>
