<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ApiError,
  getSummary,
  listCategories,
  type CategoryGroup,
  type SummaryResponse,
} from '../api/client'

type Mode = 'month' | 'year' | 'custom'

const router = useRouter()
const today = new Date().toISOString().slice(0, 10)

const mode = ref<Mode>('month')
const month = ref(today.slice(0, 7))
const year = ref(String(new Date().getFullYear()))
const fromInput = ref(today.slice(0, 8) + '01')
const toInput = ref(today)

const groups = ref<CategoryGroup[]>([])
const selected = ref<string[]>([])

const loading = ref(true)
const fetching = ref(false)
const loadError = ref('')
const filterError = ref('')
const data = ref<SummaryResponse | null>(null)

const expenseGroups = computed(() => groups.value.filter((g) => g.kind === 'expense'))

function categoryLabel(row: { category_name: string; parent_name?: string }): string {
  return row.parent_name ? `${row.parent_name} → ${row.category_name}` : row.category_name
}

function message(e: unknown): string {
  if (e instanceof ApiError && e.status === 400) return 'Check the dates and categories.'
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

function lastDayOfMonth(ym: string): string {
  const [y, m] = ym.split('-').map(Number)
  // Calendar day, never UTC: toISOString would shift back a day in CET.
  const last = new Date(y, m, 0).getDate()
  return `${ym}-${String(last).padStart(2, '0')}`
}

function resolveRange(): { from: string; to: string } | null {
  if (mode.value === 'month') {
    if (!/^\d{4}-\d{2}$/.test(month.value)) return null
    return { from: `${month.value}-01`, to: lastDayOfMonth(month.value) }
  }
  if (mode.value === 'year') {
    if (!/^\d{4}$/.test(year.value)) return null
    return { from: `${year.value}-01-01`, to: `${year.value}-12-31` }
  }
  if (!fromInput.value || !toInput.value) return null
  return { from: fromInput.value, to: toInput.value }
}

async function fetchSummary(): Promise<void> {
  filterError.value = ''
  const range = resolveRange()
  if (!range) {
    filterError.value = 'Enter a valid period.'
    return
  }
  fetching.value = true
  try {
    data.value = await getSummary(range.from, range.to, selected.value)
  } catch (e) {
    await handle(e, filterError)
  } finally {
    fetching.value = false
  }
}

onMounted(async () => {
  try {
    groups.value = await listCategories()
  } catch (e) {
    await handle(e, loadError)
  } finally {
    loading.value = false
  }
  await fetchSummary()
})
</script>

<template>
  <div class="app-main stack">
    <h1>Summary</h1>
    <p v-if="loadError" class="alert alert-error" role="alert">{{ loadError }}</p>

    <section class="card stack">
      <h2>Filters</h2>
      <form class="stack" @submit.prevent="fetchSummary">
        <label class="field">
          <span>Period</span>
          <select v-model="mode">
            <option value="month">Month</option>
            <option value="year">Year</option>
            <option value="custom">Custom range</option>
          </select>
        </label>
        <label v-if="mode === 'month'" class="field">
          <span>Month</span>
          <input v-model="month" type="month" required />
        </label>
        <label v-if="mode === 'year'" class="field">
          <span>Year</span>
          <input v-model="year" type="number" min="2000" max="2100" required />
        </label>
        <template v-if="mode === 'custom'">
          <label class="field">
            <span>From</span>
            <input v-model="fromInput" type="date" required />
          </label>
          <label class="field">
            <span>To</span>
            <input v-model="toInput" type="date" required />
          </label>
        </template>
        <fieldset v-if="!loading && expenseGroups.length" class="field">
          <legend class="muted">Categories (none selected = all)</legend>
          <div v-for="g in expenseGroups" :key="g.id" class="stack">
            <p class="muted">{{ g.name }}</p>
            <label v-if="!g.children.length" class="check">
              <input v-model="selected" type="checkbox" :value="g.id" /> {{ g.name }}
            </label>
            <label v-for="c in g.children" :key="c.id" class="check">
              <input v-model="selected" type="checkbox" :value="c.id" /> {{ c.name }}
            </label>
          </div>
        </fieldset>
        <p v-if="filterError" class="alert alert-error" role="alert">{{ filterError }}</p>
        <button type="submit" class="btn btn-primary" :disabled="fetching">Show</button>
      </form>
    </section>

    <section class="card stack">
      <h2>Totals by category</h2>
      <p v-if="fetching" class="muted">Loading…</p>
      <template v-else-if="data">
        <p v-if="data.rows.length === 0" class="muted">No expenses in this period.</p>
        <table v-else class="table">
          <thead>
            <tr>
              <th>Category</th>
              <th>Total</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in data.rows" :key="r.category_id">
              <td>{{ categoryLabel(r) }}</td>
              <td>{{ r.total }}</td>
            </tr>
            <tr>
              <td><strong>Razem</strong></td>
              <td>
                <strong>{{ data.total }}</strong>
              </td>
            </tr>
          </tbody>
        </table>
      </template>
    </section>

    <section class="card stack">
      <h2>Latest operations</h2>
      <template v-if="data && data.items.length">
        <table class="table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Category</th>
              <th>Description</th>
              <th>Amount</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in data.items" :key="t.id">
              <td>{{ t.occurred_on }}</td>
              <td>{{ categoryLabel(t) }}</td>
              <td>{{ t.description }}</td>
              <td>{{ t.amount }}</td>
            </tr>
          </tbody>
        </table>
        <p class="muted"><router-link to="/operations">See all on Operations</router-link></p>
      </template>
      <p v-else-if="data" class="muted">Nothing to show.</p>
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

fieldset {
  border: 0;
  padding: 0;
  margin: 0;
}
</style>
