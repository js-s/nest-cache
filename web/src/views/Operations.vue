<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ApiError,
  createTransaction,
  listCategories,
  listTransactions,
  type CategoryGroup,
  type Transaction,
} from '../api/client'

const LIMIT = 20
const LAST_CATEGORY_BASE = 'nestcash:last-category'

type EntryKind = 'expense' | 'income'
type ListKind = 'all' | EntryKind

const router = useRouter()

const groups = ref<CategoryGroup[]>([])
const items = ref<Transaction[]>([])
const page = ref(1)
const total = ref(0)

const loading = ref(true)
const loadingMore = ref(false)
const busy = ref(false)
const loadError = ref('')
const formError = ref('')
const listError = ref('')

const kind = ref<EntryKind>('expense')
const listKind = ref<ListKind>('all')
const categoryId = ref('')
const amount = ref('')
const occurredOn = ref(new Date().toISOString().slice(0, 10))
const description = ref('')

const kindGroups = computed(() => groups.value.filter((g) => g.kind === kind.value))

// Selectable ids: groups with no children act as a leaf; otherwise
// each child is selectable. Applies to both expense hierarchy and flat income groups.
const categoryIds = computed(() => {
  const ids = new Set<string>()
  for (const g of kindGroups.value) {
    if (g.children.length) {
      for (const c of g.children) ids.add(c.id)
    } else {
      ids.add(g.id)
    }
  }
  return ids
})

const hasCategories = computed(() => categoryIds.value.size > 0)
const hasMore = computed(() => items.value.length < total.value)

function lastKey(k: EntryKind): string {
  return `${LAST_CATEGORY_BASE}:${k}`
}

function categoryLabel(t: Transaction): string {
  return t.parent_name ? `${t.parent_name} → ${t.category_name}` : t.category_name
}

function message(e: unknown): string {
  if (e instanceof ApiError && e.status === 400) return 'Check the amount, category, and date.'
  return 'Something went wrong. Please try again.'
}

// ponytail: no global 401 interceptor yet; redirect locally on session loss.
async function handle(e: unknown, target: { value: string }): Promise<void> {
  if (e instanceof ApiError && e.status === 401) {
    await router.push('/login')
    return
  }
  target.value = message(e)
}

async function loadFirstPage(): Promise<void> {
  listError.value = ''
  loading.value = true
  try {
    const res = await listTransactions(1, LIMIT, listKind.value)
    items.value = res.items
    page.value = res.page
    total.value = res.total
  } catch (e) {
    await handle(e, listError)
  } finally {
    loading.value = false
  }
}

async function loadMore(): Promise<void> {
  listError.value = ''
  loadingMore.value = true
  try {
    const res = await listTransactions(page.value + 1, LIMIT, listKind.value)
    items.value = items.value.concat(res.items)
    page.value = res.page
    total.value = res.total
  } catch (e) {
    // Keep already-loaded items; only surface the error.
    await handle(e, listError)
  } finally {
    loadingMore.value = false
  }
}

function restoreLastCategory(): void {
  const saved = localStorage.getItem(lastKey(kind.value))
  if (saved && categoryIds.value.has(saved)) {
    categoryId.value = saved
  }
}

// Switching entry kind resets the category: ids from one kind never exist in the other.
function switchKind(next: EntryKind): void {
  if (kind.value === next) return
  kind.value = next
  categoryId.value = ''
  formError.value = ''
  restoreLastCategory()
}

onMounted(async () => {
  try {
    const cats = await listCategories()
    groups.value = cats
    restoreLastCategory()
  } catch (e) {
    await handle(e, loadError)
  }
  await loadFirstPage()
})

// Mirrors the backend amount rule so bad input gets an explicit message
// instead of a silent native-validation block (Polish "12,34" included).
const amountRe = /^\d{1,10}(\.\d{1,2})?$/

async function submit(): Promise<void> {
  formError.value = ''
  if (!categoryId.value) {
    formError.value = 'Select a category.'
    return
  }
  const value = amount.value.trim().replace(',', '.')
  if (!amountRe.test(value) || !/[1-9]/.test(value)) {
    formError.value = 'Enter a positive amount, e.g. 12.34.'
    return
  }
  busy.value = true
  try {
    await createTransaction({
      amount: value,
      category_id: categoryId.value,
      occurred_on: occurredOn.value,
      description: description.value.trim() || undefined,
      kind: kind.value,
    })
    localStorage.setItem(lastKey(kind.value), categoryId.value)
    amount.value = ''
    description.value = ''
    // ponytail: occurred_on may be older than the newest row, so reload page 1
    // instead of prepending (which would break newest-first ordering).
    await loadFirstPage()
  } catch (e) {
    await handle(e, formError)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="app-main stack">
    <h1>Operations</h1>
    <p v-if="loadError" class="alert alert-error" role="alert">{{ loadError }}</p>

    <section class="card stack">
      <h2>Add operation</h2>
      <div class="field" role="radiogroup" aria-label="Type">
        <label class="check">
          <input type="radio" :checked="kind === 'expense'" @change="switchKind('expense')" /> Expense
        </label>
        <label class="check">
          <input type="radio" :checked="kind === 'income'" @change="switchKind('income')" /> Income
        </label>
      </div>
      <p v-if="!loading && !hasCategories" class="alert alert-error" role="alert">
        No {{ kind }} categories yet. <router-link to="/categories">Manage categories</router-link>.
      </p>
      <form class="stack" @submit.prevent="submit">
        <label class="field">
          <span>Category</span>
          <select v-model="categoryId" :disabled="!hasCategories">
            <option value="" disabled>Select a category…</option>
            <optgroup v-for="g in kindGroups" :key="g.id" :label="g.name">
              <template v-if="g.children.length">
                <option v-for="c in g.children" :key="c.id" :value="c.id">{{ c.name }}</option>
              </template>
              <option v-else :value="g.id">{{ g.name }}</option>
            </optgroup>
          </select>
        </label>
        <label class="field">
          <span>Amount</span>
          <input v-model="amount" type="text" inputmode="decimal" autocomplete="off" placeholder="0.00" />
        </label>
        <label class="field">
          <span>Date</span>
          <input v-model="occurredOn" type="date" required />
        </label>
        <label class="field">
          <span>Description (optional)</span>
          <input v-model="description" type="text" maxlength="500" />
        </label>
        <p v-if="formError" class="alert alert-error" role="alert">{{ formError }}</p>
        <button type="submit" class="btn btn-primary" :disabled="busy || !hasCategories">Save</button>
      </form>
    </section>

    <section class="card stack">
      <h2>Operations</h2>
      <label class="field">
        <span>Show</span>
        <select v-model="listKind" @change="loadFirstPage">
          <option value="all">All</option>
          <option value="expense">Expenses</option>
          <option value="income">Incomes</option>
        </select>
      </label>
      <p v-if="loading" class="muted">Loading…</p>
      <template v-else>
        <p v-if="listError" class="alert alert-error" role="alert">{{ listError }}</p>
        <p v-if="items.length === 0" class="muted">No operations yet.</p>
        <table v-else class="table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Type</th>
              <th>Category</th>
              <th>Description</th>
              <th>Amount</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in items" :key="t.id">
              <td>{{ t.occurred_on }}</td>
              <td>{{ t.kind }}</td>
              <td>{{ categoryLabel(t) }}</td>
              <td>{{ t.description }}</td>
              <td>{{ t.amount }}</td>
            </tr>
          </tbody>
        </table>
        <button
          v-if="hasMore"
          type="button"
          class="btn btn-secondary"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : 'Load more' }}
        </button>
      </template>
    </section>
  </div>
</template>

<style scoped>
.check {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 400;
}

.check input {
  width: auto;
  min-height: auto;
}
</style>
