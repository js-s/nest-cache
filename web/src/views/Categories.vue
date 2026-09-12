<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ApiError, createCategory, listCategories, type CategoryGroup } from '../api/client'

const groups = ref<CategoryGroup[]>([])
const loadError = ref('')
const formError = ref('')
const busy = ref(false)
const loading = ref(true)

const kind = ref<'expense' | 'income'>('expense')
const groupSel = ref('')
const name = ref('')

const NEW_GROUP = '__new__'

const kindGroups = computed(() => groups.value.filter((g) => g.kind === kind.value))

const isNewGroup = computed(() => groupSel.value === NEW_GROUP || kindGroups.value.length === 0)

// ponytail: stale group selection across type switch would 400; reset it.
watch(kind, () => {
  groupSel.value = ''
})

function message(e: unknown): string {
  if (e instanceof ApiError && e.status === 409) return 'Such a category already exists.'
  if (e instanceof ApiError && e.status === 400)
    return 'Check the data: name 1–80 characters, valid type and group.'
  return 'Saving failed. Please try again.'
}

async function load(): Promise<void> {
  loadError.value = ''
  loading.value = true
  try {
    groups.value = await listCategories()
  } catch (e) {
    loadError.value = e instanceof ApiError ? message(e) : 'Loading failed. Please try again.'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})

async function submit(): Promise<void> {
  formError.value = ''
  const label = name.value.trim()
  if (!label) {
    formError.value = 'Enter a name.'
    return
  }
  if (!isNewGroup.value && !groupSel.value) {
    formError.value = 'Select a group.'
    return
  }
  busy.value = true
  try {
    if (isNewGroup.value) {
      await createCategory({ kind: kind.value, name: label })
    } else {
      await createCategory({ kind: kind.value, name: label, parent_id: groupSel.value })
    }
    name.value = ''
    await load()
  } catch (e) {
    // ponytail: keep typed input on error; clear only on success above.
    formError.value = message(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="app-main stack">
    <h1>Categories</h1>
    <p v-if="loadError" class="alert alert-error" role="alert">{{ loadError }}</p>
    <p v-if="loading" class="muted">Loading…</p>
    <template v-else>
      <section v-for="g in groups" :key="g.id" class="card stack">
        <h2>{{ g.name }} <span class="muted">({{ g.kind }})</span></h2>
        <table v-if="g.children.length" class="table">
          <tbody>
            <tr v-for="c in g.children" :key="c.id">
              <td>{{ c.name }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">No subcategories yet.</p>
      </section>
    </template>
    <section class="card stack">
      <h2>Add a category</h2>
      <form class="stack" @submit.prevent="submit">
        <label class="field">
          <span>Type</span>
          <select v-model="kind">
            <option value="expense">Expense</option>
            <option value="income">Income</option>
          </select>
        </label>
        <label v-if="kindGroups.length" class="field">
          <span>Group</span>
          <select v-model="groupSel">
            <option value="" disabled>Select a group…</option>
            <option v-for="g in kindGroups" :key="g.id" :value="g.id">{{ g.name }}</option>
            <option :value="NEW_GROUP">New group…</option>
          </select>
        </label>
        <label class="field">
          <span>{{ isNewGroup ? 'New group name' : 'Subcategory name' }}</span>
          <input v-model="name" type="text" maxlength="80" required />
        </label>
        <p v-if="formError" class="alert alert-error" role="alert">{{ formError }}</p>
        <button type="submit" class="btn btn-primary" :disabled="busy">Save</button>
      </form>
    </section>
  </div>
</template>
