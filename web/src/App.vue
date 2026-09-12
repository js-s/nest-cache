<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ensureSession, logoutSession, useSession } from './auth'
import { theme, toggleTheme } from './theme'

const router = useRouter()
const { user } = useSession()

onMounted(() => {
  void ensureSession()
})

async function logout(): Promise<void> {
  await logoutSession()
  await router.push('/login')
}
</script>

<template>
  <header class="app-header">
    <div class="page app-header__inner">
      <span class="brand">NestCash</span>
      <div class="app-header__user">
        <span v-if="user" class="user-email">{{ user.email }}</span>
        <button type="button" class="btn btn-secondary" @click="toggleTheme">
          {{ theme === 'dark' ? 'Light mode' : 'Dark mode' }}
        </button>
        <button v-if="user" type="button" class="btn btn-secondary" @click="logout">Log out</button>
      </div>
    </div>
  </header>
  <main class="page">
    <router-view />
  </main>
</template>
