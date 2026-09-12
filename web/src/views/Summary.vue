<script setup lang="ts">
import { useSession } from '../auth'

const { user } = useSession()
</script>

<template>
  <div class="app-main stack">
    <h1>Summary</h1>
    <p v-if="user" class="muted">Signed in as {{ user.email }}.</p>
    <p><router-link to="/categories">Manage categories</router-link></p>
    <p class="muted">Budget tracking lands in the next slice — session guard verified.</p>
  </div>
</template>
