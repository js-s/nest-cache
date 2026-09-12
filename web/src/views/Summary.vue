<script setup lang="ts">
import { useSession } from '../auth'

const { user } = useSession()
</script>

<template>
  <main>
    <h1>Summary</h1>
    <p v-if="user">Signed in as {{ user.email }}.</p>
    <p>Budget tracking lands in the next slice — session guard verified.</p>
  </main>
</template>
