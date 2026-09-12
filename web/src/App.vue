<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ensureSession, logoutSession, useSession } from './auth'

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
  <header>
    <nav>
      <span>NestCash</span>
      <template v-if="user">
        <span>{{ user.email }}</span>
        <button type="button" @click="logout">Log out</button>
      </template>
    </nav>
  </header>
  <router-view />
</template>
