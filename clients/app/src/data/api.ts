import { USD, dinero, toDecimal, toSnapshot } from 'dinero.js'
import type { Dinero } from 'dinero.js'
import { getAccessToken } from '#/auth/msalConfig'

const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

export type Money = Dinero<number>

export interface Income {
  id: string
  date: string
  amount: Money
  ledgerId: string | null
  notes: string | null
  createdAt: string
  updatedAt: string
}

export interface IncomeInput {
  date: string
  amount: Money
  ledgerId: string | null
  notes: string | null
}

interface RawMoney {
  amount: number
  currency: string
}

interface RawIncome {
  id: string
  date: string
  amount: RawMoney
  ledgerId: string | null
  notes: string | null
  createdAt: string
  updatedAt: string
}

function toMoney(raw: RawMoney): Money {
  if (raw.currency !== 'USD') {
    throw new Error(`unsupported currency: ${raw.currency}`)
  }
  return dinero({ amount: raw.amount, currency: USD })
}

function toRawMoney(amount: Money): RawMoney {
  const snapshot = toSnapshot(amount)
  return { amount: snapshot.amount, currency: snapshot.currency.code }
}

function toIncome(raw: RawIncome): Income {
  return { ...raw, amount: toMoney(raw.amount) }
}

function toIncomeBody(input: IncomeInput) {
  return {
    date: input.date,
    amount: toRawMoney(input.amount),
    ledgerId: input.ledgerId,
    notes: input.notes,
  }
}

// moneyToNumber is for display only (formatCurrency needs a plain number) —
// never used for storage or wire transfer, where Money stays exact.
export function moneyToNumber(amount: Money): number {
  return Number(toDecimal(amount))
}

// numberToMoney is for turning a form input's typed dollar amount into a
// Money before it ever touches store/API code.
export function numberToMoney(dollars: number): Money {
  return dinero({ amount: Math.round(dollars * 100), currency: USD })
}

// Tiny external store tracking in-flight requests, so any part of the app
// (LoadingOverlay) can show a loading state without threading it through
// every call site. Subscribe via React's useSyncExternalStore.
const loadingListeners = new Set<() => void>()
let inFlightCount = 0

function setInFlight(delta: number) {
  inFlightCount += delta
  loadingListeners.forEach((listener) => listener())
}

export function subscribeLoading(listener: () => void): () => void {
  loadingListeners.add(listener)
  return () => loadingListeners.delete(listener)
}

export function getLoadingSnapshot(): boolean {
  return inFlightCount > 0
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  setInFlight(1)
  try {
    return await requestOnce<T>(path, init)
  } finally {
    setInFlight(-1)
  }
}

async function fetchWithToken(
  path: string,
  init: RequestInit | undefined,
  token: string,
): Promise<Response> {
  return fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })
}

async function requestOnce<T>(path: string, init?: RequestInit): Promise<T> {
  const token = await getAccessToken()
  let res = await fetchWithToken(path, init, token)
  if (res.status === 401) {
    // The cached token MSAL handed us didn't actually work against the
    // API — force a fresh one and retry once before giving up, instead of
    // failing every call until the user manually reloads.
    const refreshedToken = await getAccessToken({ forceRefresh: true })
    res = await fetchWithToken(path, init, refreshedToken)
  }
  if (!res.ok) {
    throw new Error(`${init?.method ?? 'GET'} ${path} failed: ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export interface IncomeListFilter {
  year?: number
  from?: string
  to?: string
}

export async function listIncomes(
  filter?: IncomeListFilter,
): Promise<Income[]> {
  const params = new URLSearchParams()
  if (filter?.year != null) params.set('year', String(filter.year))
  if (filter?.from) params.set('from', filter.from)
  if (filter?.to) params.set('to', filter.to)
  const query = params.size > 0 ? `?${params.toString()}` : ''

  const raw = await request<RawIncome[]>(`/incomes${query}`)
  return raw.map(toIncome)
}

export async function createIncome(input: IncomeInput): Promise<Income> {
  const raw = await request<RawIncome>('/incomes', {
    method: 'POST',
    body: JSON.stringify(toIncomeBody(input)),
  })
  return toIncome(raw)
}

export async function updateIncome(
  id: string,
  input: IncomeInput,
): Promise<Income> {
  const raw = await request<RawIncome>(`/incomes/${id}`, {
    method: 'PUT',
    body: JSON.stringify(toIncomeBody(input)),
  })
  return toIncome(raw)
}

export async function deleteIncome(id: string): Promise<void> {
  await request<void>(`/incomes/${id}`, { method: 'DELETE' })
}

export interface Bill {
  id: string
  name: string
  amount: Money
  dueDay: number | null
  isAutoPay: boolean
  notes: string | null
  createdAt: string
  updatedAt: string
}

export interface BillInput {
  name: string
  amount: Money
  dueDay: number | null
  isAutoPay: boolean
  notes: string | null
}

interface RawBill {
  id: string
  name: string
  amount: RawMoney
  dueDay: number | null
  isAutoPay: boolean
  notes: string | null
  createdAt: string
  updatedAt: string
}

function toBill(raw: RawBill): Bill {
  return { ...raw, amount: toMoney(raw.amount) }
}

function toBillBody(input: BillInput) {
  return {
    name: input.name,
    amount: toRawMoney(input.amount),
    dueDay: input.dueDay,
    isAutoPay: input.isAutoPay,
    notes: input.notes,
  }
}

export async function listBills(): Promise<Bill[]> {
  const raw = await request<RawBill[]>('/bills')
  return raw.map(toBill)
}

export async function createBill(input: BillInput): Promise<Bill> {
  const raw = await request<RawBill>('/bills', {
    method: 'POST',
    body: JSON.stringify(toBillBody(input)),
  })
  return toBill(raw)
}

export async function updateBill(id: string, input: BillInput): Promise<Bill> {
  const raw = await request<RawBill>(`/bills/${id}`, {
    method: 'PUT',
    body: JSON.stringify(toBillBody(input)),
  })
  return toBill(raw)
}

export async function deleteBill(id: string): Promise<void> {
  await request<void>(`/bills/${id}`, { method: 'DELETE' })
}

export interface Ledger {
  id: string
  date: string
  name: string | null
  bankBalance: Money
  income: Money
  expenses: Money
  net: Money
  total: Money | null
  notes: string | null
  createdAt: string
  updatedAt: string
}

export interface LedgerInput {
  date: string
  name: string | null
  bankBalance?: Money
  income?: Money
  expenses?: Money
  notes?: string | null
}

export interface LedgerBill {
  id: string
  ledgerId: string
  billId: string
  amount: Money
  dueDay: number | null
  isPayed: boolean
  notes: string | null
  createdAt: string
  updatedAt: string
}

export interface LedgerBillInput {
  billId: string
  amount: Money
  dueDay: number | null
  isPayed: boolean
  notes: string | null
}

export interface LedgerBillWithBill extends LedgerBill {
  bill: Bill
}

export interface LedgerDetail extends Ledger {
  incomes: Income[]
  ledgerBills: LedgerBillWithBill[]
}

interface RawLedger {
  id: string
  date: string
  name: string | null
  bankBalance: RawMoney
  income: RawMoney
  expenses: RawMoney
  net: RawMoney
  total: RawMoney | null
  notes: string | null
  createdAt: string
  updatedAt: string
}

interface RawLedgerBill {
  id: string
  ledgerId: string
  billId: string
  amount: RawMoney
  dueDay: number | null
  isPayed: boolean
  notes: string | null
  createdAt: string
  updatedAt: string
}

interface RawLedgerDetail extends RawLedger {
  incomes: RawIncome[]
  ledgerBills: (RawLedgerBill & { bill: RawBill })[]
}

function toLedger(raw: RawLedger): Ledger {
  return {
    ...raw,
    bankBalance: toMoney(raw.bankBalance),
    income: toMoney(raw.income),
    expenses: toMoney(raw.expenses),
    net: toMoney(raw.net),
    total: raw.total ? toMoney(raw.total) : null,
  }
}

function toLedgerBill(raw: RawLedgerBill): LedgerBill {
  return { ...raw, amount: toMoney(raw.amount) }
}

function toLedgerDetail(raw: RawLedgerDetail): LedgerDetail {
  return {
    ...toLedger(raw),
    incomes: raw.incomes.map(toIncome),
    ledgerBills: raw.ledgerBills.map((lb) => ({
      ...toLedgerBill(lb),
      bill: toBill(lb.bill),
    })),
  }
}

function toLedgerBody(input: LedgerInput) {
  return {
    date: input.date,
    name: input.name,
    bankBalance: input.bankBalance ? toRawMoney(input.bankBalance) : undefined,
    income: input.income ? toRawMoney(input.income) : undefined,
    expenses: input.expenses ? toRawMoney(input.expenses) : undefined,
    notes: input.notes,
  }
}

function toLedgerBillBody(input: LedgerBillInput) {
  return {
    billId: input.billId,
    amount: toRawMoney(input.amount),
    dueDay: input.dueDay,
    isPayed: input.isPayed,
    notes: input.notes,
  }
}

export async function listLedgers(): Promise<Ledger[]> {
  const raw = await request<RawLedger[]>('/ledgers')
  return raw.map(toLedger)
}

export async function getLedger(id: string): Promise<LedgerDetail> {
  const raw = await request<RawLedgerDetail>(`/ledgers/${id}`)
  return toLedgerDetail(raw)
}

export async function createLedger(input: LedgerInput): Promise<Ledger> {
  const raw = await request<RawLedger>('/ledgers', {
    method: 'POST',
    body: JSON.stringify(toLedgerBody(input)),
  })
  return toLedger(raw)
}

export async function updateLedger(
  id: string,
  input: LedgerInput,
): Promise<Ledger> {
  const raw = await request<RawLedger>(`/ledgers/${id}`, {
    method: 'PUT',
    body: JSON.stringify(toLedgerBody(input)),
  })
  return toLedger(raw)
}

export async function deleteLedger(id: string): Promise<void> {
  await request<void>(`/ledgers/${id}`, { method: 'DELETE' })
}

export async function createLedgerBill(
  ledgerId: string,
  input: LedgerBillInput,
): Promise<LedgerBill> {
  const raw = await request<RawLedgerBill>(`/ledgers/${ledgerId}/bills`, {
    method: 'POST',
    body: JSON.stringify(toLedgerBillBody(input)),
  })
  return toLedgerBill(raw)
}

export async function updateLedgerBill(
  ledgerId: string,
  id: string,
  input: LedgerBillInput,
): Promise<LedgerBill> {
  const raw = await request<RawLedgerBill>(`/ledgers/${ledgerId}/bills/${id}`, {
    method: 'PUT',
    body: JSON.stringify(toLedgerBillBody(input)),
  })
  return toLedgerBill(raw)
}

export async function deleteLedgerBill(
  ledgerId: string,
  id: string,
): Promise<void> {
  await request<void>(`/ledgers/${ledgerId}/bills/${id}`, {
    method: 'DELETE',
  })
}

export interface CurrentLedger {
  id: string
  date: string
  name: string | null
  availableFunds: string
  paid: string
  planned: string
  net: string
  unpaidCount: number
  checkIn: { status: 'good' | 'tight' | 'negative' }
}

export interface LedgerHistoryEntry {
  id: string
  date: string
  name: string | null
  net: string
}

export async function getCurrentLedger(): Promise<CurrentLedger | null> {
  try {
    return await request<CurrentLedger>('/ledgers/current')
  } catch (err) {
    if (err instanceof Error && err.message.includes('404')) return null
    throw err
  }
}

export async function getLedgerHistory(
  limit?: number,
): Promise<LedgerHistoryEntry[]> {
  const query = limit ? `?limit=${limit}` : ''
  return request<LedgerHistoryEntry[]>(`/ledgers/history${query}`)
}

export interface Pto {
  id: string
  year: number
  prevYearHours: string
  availableHours: string
  hoursPlanned: string
  hoursUsed: string
  hoursRemaining: string
  rolloverHours: boolean
  createdAt: string
  updatedAt: string
}

export interface PtoInput {
  year: number
  prevYearHours: string
  availableHours: string
  rolloverHours: boolean
}

export type PtoPlanStatus = 'Planned' | 'Completed'

export interface PtoPlan {
  id: string
  ptoId: string
  startDate: string
  endDate: string
  name: string
  description: string | null
  hours: string
  status: PtoPlanStatus
  customHours: boolean
  createdAt: string
  updatedAt: string
}

export interface PtoPlanInput {
  startDate: string
  endDate: string
  name: string
  description: string | null
  hours: string
  status: PtoPlanStatus
  customHours: boolean
}

export interface HolidayHour {
  id: string
  ptoId: string
  date: string
  name: string
  hours: string
  createdAt: string
  updatedAt: string
}

export interface HolidayHourInput {
  date: string
  name: string
  hours: string
}

export interface PtoDetail extends Pto {
  ptoPlans: PtoPlan[]
  holidayHours: HolidayHour[]
}

export async function listPtos(): Promise<Pto[]> {
  return request<Pto[]>('/ptos')
}

export async function getCurrentPto(): Promise<Pto | null> {
  try {
    return await request<Pto>('/ptos/current')
  } catch (err) {
    if (err instanceof Error && err.message.includes('404')) return null
    throw err
  }
}

export async function getPto(id: string): Promise<PtoDetail> {
  return request<PtoDetail>(`/ptos/${id}`)
}

export async function createPto(input: PtoInput): Promise<Pto> {
  return request<Pto>('/ptos', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function updatePto(id: string, input: PtoInput): Promise<Pto> {
  return request<Pto>(`/ptos/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deletePto(id: string): Promise<void> {
  await request<void>(`/ptos/${id}`, { method: 'DELETE' })
}

export async function createPtoPlan(
  ptoId: string,
  input: PtoPlanInput,
): Promise<PtoPlan> {
  return request<PtoPlan>(`/ptos/${ptoId}/plans`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function updatePtoPlan(
  ptoId: string,
  id: string,
  input: PtoPlanInput,
): Promise<PtoPlan> {
  return request<PtoPlan>(`/ptos/${ptoId}/plans/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deletePtoPlan(ptoId: string, id: string): Promise<void> {
  await request<void>(`/ptos/${ptoId}/plans/${id}`, { method: 'DELETE' })
}

export async function createHolidayHour(
  ptoId: string,
  input: HolidayHourInput,
): Promise<HolidayHour> {
  return request<HolidayHour>(`/ptos/${ptoId}/holidays`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function updateHolidayHour(
  ptoId: string,
  id: string,
  input: HolidayHourInput,
): Promise<HolidayHour> {
  return request<HolidayHour>(`/ptos/${ptoId}/holidays/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export async function deleteHolidayHour(
  ptoId: string,
  id: string,
): Promise<void> {
  await request<void>(`/ptos/${ptoId}/holidays/${id}`, { method: 'DELETE' })
}
