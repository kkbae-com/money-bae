import { createContext, useContext, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useMsal } from '@azure/msal-react'
import { loginRequest } from '#/auth/msalConfig'
import {
  createBill,
  createHolidayHour,
  createIncome,
  createLedger,
  createLedgerBill,
  createPto,
  createPtoPlan,
  deleteBill,
  deleteHolidayHour,
  deleteIncome as apiDeleteIncome,
  deleteLedger,
  deleteLedgerBill,
  deletePto,
  deletePtoPlan,
  listBills,
  listIncomes,
  listLedgers,
  listPtos,
  updateBill,
  updateHolidayHour,
  updateIncome as apiUpdateIncome,
  updateLedger,
  updateLedgerBill,
  updatePto,
  updatePtoPlan,
} from '#/data/api'
import type {
  Bill,
  BillInput,
  HolidayHour,
  HolidayHourInput,
  Income,
  IncomeInput,
  Ledger,
  LedgerBillWithBill,
  LedgerBillInput,
  LedgerInput,
  Pto,
  PtoInput,
  PtoPlan,
  PtoPlanInput,
} from '#/data/api'

type ModalKind =
  | 'bill'
  | 'income'
  | 'ledger'
  | 'ledgerBill'
  | 'pto'
  | 'ptoYear'
  | 'holiday'
  | null
type ModalMode = 'Add' | 'Edit'

export type ToastKind = 'error' | 'warning' | 'info'

export interface ToastMessage {
  id: string
  kind: ToastKind
  text: string
}

// info toasts are good-news/success acks (e.g. "duplicated that bill") —
// glance-and-move-on. error/warning ones report something that needs
// attention (a failed save, a required selection), so they get longer to
// actually read before they auto-dismiss.
const TOAST_DURATION_MS: Record<ToastKind, number> = {
  info: 500,
  warning: 5000,
  error: 6000,
}

export interface PendingDelete {
  count: number
  onConfirm: () => void
}

interface AppStore {
  authed: boolean
  email: string | null
  accountOpen: boolean
  bills: Bill[]
  billSelected: string | null
  ledgers: Ledger[]
  ledgerSelected: string | null
  activeLedgerId: string | null
  selectedLedgerBill: LedgerBillWithBill | null
  ptos: Pto[]
  ptoYearSelected: string | null
  income: Income[]
  incomeSelected: string | null
  activePtoId: string | null
  selectedPtoPlan: PtoPlan | null
  selectedHoliday: HolidayHour | null
  toasts: ToastMessage[]
  pendingDelete: PendingDelete | null
  modal: ModalKind
  modalMode: ModalMode
  ensureIncome: () => Promise<void>
  ensureBills: () => Promise<void>
  loadLedgers: () => Promise<void>
  ensurePtos: () => Promise<void>
  signIn: () => void
  signOut: () => void
  toggleAccountMenu: () => void
  selectBill: (id: string | null) => void
  addBillEntry: (input: BillInput) => Promise<boolean>
  editBillEntry: (input: BillInput) => Promise<boolean>
  duplicateBillEntry: (id: string) => Promise<void>
  deleteBillEntries: (ids: string[]) => Promise<void>
  selectLedger: (id: string | null) => void
  addLedgerEntry: (input: LedgerInput) => Promise<boolean>
  editLedgerEntry: (input: LedgerInput) => Promise<boolean>
  duplicateLedgerEntry: (id: string) => Promise<void>
  deleteLedgerEntries: (ids: string[]) => Promise<void>
  deleteLedgerBillEntries: (ledgerId: string, ids: string[]) => Promise<void>
  setActiveLedger: (id: string | null) => void
  selectLedgerBill: (entry: LedgerBillWithBill | null) => void
  addLedgerBillEntry: (input: LedgerBillInput) => Promise<boolean>
  editLedgerBillEntry: (input: LedgerBillInput) => Promise<boolean>
  selectIncome: (id: string | null) => void
  addIncomeEntry: (input: IncomeInput) => Promise<boolean>
  editIncomeEntry: (input: IncomeInput) => Promise<boolean>
  deleteIncomeEntries: (ids: string[]) => Promise<void>
  duplicateIncomeEntry: (id: string) => Promise<void>
  attachIncomesToLedger: (
    ledgerId: string | null,
    ids: string[],
  ) => Promise<void>
  showToast: (kind: ToastKind, text: string) => void
  dismissToast: (id: string) => void
  requestDelete: (count: number, onConfirm: () => void) => void
  confirmDelete: () => void
  cancelDelete: () => void
  selectPtoYear: (id: string | null) => void
  addPtoYearEntry: (input: PtoInput) => Promise<boolean>
  editPtoYearEntry: (input: PtoInput) => Promise<boolean>
  duplicatePtoYearEntry: (id: string) => Promise<void>
  deletePtoYearEntries: (ids: string[]) => Promise<void>
  setActivePto: (id: string | null) => void
  selectPtoPlan: (entry: PtoPlan | null) => void
  addPtoPlanEntry: (input: PtoPlanInput) => Promise<boolean>
  editPtoPlanEntry: (input: PtoPlanInput) => Promise<boolean>
  duplicatePtoPlanEntry: (input: PtoPlanInput) => Promise<PtoPlan | null>
  deletePtoPlanEntries: (ptoId: string, ids: string[]) => Promise<void>
  selectHoliday: (entry: HolidayHour | null) => void
  addHolidayEntry: (input: HolidayHourInput) => Promise<boolean>
  editHolidayEntry: (input: HolidayHourInput) => Promise<boolean>
  duplicateHolidayEntry: (
    input: HolidayHourInput,
  ) => Promise<HolidayHour | null>
  deleteHolidayEntries: (ptoId: string, ids: string[]) => Promise<void>
  openBillModal: (mode: ModalMode) => void
  openIncomeModal: (mode: ModalMode) => void
  openLedgerModal: (mode: ModalMode) => void
  openLedgerBillModal: (mode: ModalMode) => void
  openPtoYearModal: (mode: ModalMode) => void
  openPtoPlanModal: (mode: ModalMode) => void
  openHolidayModal: (mode: ModalMode) => void
  closeModal: () => void
}

const AppStoreContext = createContext<AppStore | null>(null)

export function AppStoreProvider({ children }: { children: ReactNode }) {
  const { instance, accounts } = useMsal()
  // tsconfig doesn't set noUncheckedIndexedAccess, so TS types accounts[0]
  // as always-defined — the null checks below are still needed at runtime
  // for the empty-array case (signed out).
  const account = accounts[0] ?? null
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  const authed = account !== null
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  const email = account?.username ?? null
  const [accountOpen, setAccountOpen] = useState(false)
  const [bills, setBills] = useState<Bill[]>([])
  const [billSelected, setBillSelected] = useState<string | null>(null)
  const [ledgers, setLedgers] = useState<Ledger[]>([])
  const [ledgerSelected, setLedgerSelected] = useState<string | null>(null)
  const [activeLedgerId, setActiveLedgerId] = useState<string | null>(null)
  const [selectedLedgerBill, setSelectedLedgerBill] =
    useState<LedgerBillWithBill | null>(null)
  const [ptos, setPtos] = useState<Pto[]>([])
  const [ptoYearSelected, setPtoYearSelected] = useState<string | null>(null)
  const [activePtoId, setActivePtoId] = useState<string | null>(null)
  const [selectedPtoPlan, setSelectedPtoPlan] = useState<PtoPlan | null>(null)
  const [selectedHoliday, setSelectedHoliday] = useState<HolidayHour | null>(
    null,
  )
  const [income, setIncome] = useState<Income[]>([])
  const [incomeSelected, setIncomeSelected] = useState<string | null>(null)
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null)
  const [modal, setModal] = useState<ModalKind>(null)
  const [modalMode, setModalMode] = useState<ModalMode>('Edit')

  function showToast(kind: ToastKind, text: string) {
    const id = crypto.randomUUID()
    setToasts((prev) => [...prev, { id, kind, text }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, TOAST_DURATION_MS[kind])
  }

  function dismissToast(id: string) {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  function requestDelete(count: number, onConfirm: () => void) {
    if (count === 0) {
      showToast('warning', 'select something to delete first')
      return
    }
    setPendingDelete({ count, onConfirm })
  }

  function confirmDelete() {
    pendingDelete?.onConfirm()
    setPendingDelete(null)
  }

  function cancelDelete() {
    setPendingDelete(null)
  }

  // Each list loads lazily, the first time a route that actually needs it
  // calls ensureX — not eagerly for the whole app on mount. The ref-cached
  // promise dedupes concurrent/repeat calls (e.g. two routes needing bills)
  // so it fetches at most once per session; a failed fetch clears the cache
  // so the next visit can retry.
  const incomePromiseRef = useRef<Promise<void> | null>(null)
  const billsPromiseRef = useRef<Promise<void> | null>(null)
  const ptosPromiseRef = useRef<Promise<void> | null>(null)

  function ensureIncome(): Promise<void> {
    if (incomePromiseRef.current) return incomePromiseRef.current
    const promise = listIncomes()
      .then(setIncome)
      .catch((err: unknown) => {
        console.error('failed to load incomes', err)
        showToast('error', "couldn't load your incomes")
        incomePromiseRef.current = null
      })
    incomePromiseRef.current = promise
    return promise
  }

  function ensureBills(): Promise<void> {
    if (billsPromiseRef.current) return billsPromiseRef.current
    const promise = listBills()
      .then(setBills)
      .catch((err: unknown) => {
        console.error('failed to load bills', err)
        showToast('error', "couldn't load your bills")
        billsPromiseRef.current = null
      })
    billsPromiseRef.current = promise
    return promise
  }

  // Unlike ensureIncome/ensureBills/ensurePtos above, this always refetches
  // rather than caching for the session: a ledger's net/total/income/
  // expenses are recalculated server-side whenever an Income or LedgerBill
  // attached to it changes elsewhere (the detail page, an import), so a
  // cached ledgers list goes stale the moment you leave the list page and
  // come back.
  function loadLedgers(): Promise<void> {
    return listLedgers()
      .then(setLedgers)
      .catch((err: unknown) => {
        console.error('failed to load ledgers', err)
        showToast('error', "couldn't load your ledger")
      })
  }

  function ensurePtos(): Promise<void> {
    if (ptosPromiseRef.current) return ptosPromiseRef.current
    const promise = listPtos()
      .then(setPtos)
      .catch((err: unknown) => {
        console.error('failed to load ptos', err)
        showToast('error', "couldn't load your PTO records")
        ptosPromiseRef.current = null
      })
    ptosPromiseRef.current = promise
    return promise
  }

  const store: AppStore = {
    authed,
    email,
    accountOpen,
    bills,
    billSelected,
    ledgers,
    ledgerSelected,
    activeLedgerId,
    selectedLedgerBill,
    ptos,
    ptoYearSelected,
    income,
    incomeSelected,
    activePtoId,
    selectedPtoPlan,
    selectedHoliday,
    toasts,
    pendingDelete,
    modal,
    modalMode,
    ensureIncome,
    ensureBills,
    loadLedgers,
    ensurePtos,
    signIn: () => void instance.loginRedirect(loginRequest),
    signOut: () => {
      setAccountOpen(false)
      setModal(null)
      void instance.logoutRedirect()
    },
    toggleAccountMenu: () => setAccountOpen((open) => !open),
    selectBill: (id) => setBillSelected(id),
    addBillEntry: async (input) => {
      try {
        const created = await createBill(input)
        setBills((prev) => [created, ...prev])
        return true
      } catch (err) {
        console.error('failed to create bill', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editBillEntry: async (input) => {
      if (!billSelected) return false
      try {
        const updated = await updateBill(billSelected, input)
        setBills((prev) => prev.map((b) => (b.id === updated.id ? updated : b)))
        return true
      } catch (err) {
        console.error('failed to update bill', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    duplicateBillEntry: async (id) => {
      const original = bills.find((b) => b.id === id)
      if (!original) return
      try {
        const created = await createBill({
          name: `${original.name} (copy)`,
          amount: original.amount,
          dueDay: original.dueDay,
          isAutoPay: original.isAutoPay,
          notes: original.notes,
        })
        setBills((prev) => [created, ...prev])
        showToast('info', 'duplicated that bill')
      } catch (err) {
        console.error('failed to duplicate bill', err)
        showToast('error', "couldn't duplicate — try again")
      }
    },
    deleteBillEntries: async (ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deleteBill(id)))
        setBills((prev) => prev.filter((b) => !ids.includes(b.id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that bill'
            : `deleted ${ids.length} bills`,
        )
      } catch (err) {
        console.error('failed to delete bill', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    selectLedger: (id) => setLedgerSelected(id),
    addLedgerEntry: async (input) => {
      try {
        const created = await createLedger(input)
        setLedgers((prev) => [created, ...prev])
        return true
      } catch (err) {
        console.error('failed to create ledger', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editLedgerEntry: async (input) => {
      if (!ledgerSelected) return false
      try {
        const updated = await updateLedger(ledgerSelected, input)
        setLedgers((prev) =>
          prev.map((l) => (l.id === updated.id ? updated : l)),
        )
        return true
      } catch (err) {
        console.error('failed to update ledger', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    duplicateLedgerEntry: async (id) => {
      const original = ledgers.find((l) => l.id === id)
      if (!original) return
      try {
        const created = await createLedger({
          date: original.date,
          name: original.name ? `${original.name} (copy)` : null,
          bankBalance: original.bankBalance,
          income: original.income,
          expenses: original.expenses,
          notes: original.notes,
        })
        setLedgers((prev) => [created, ...prev])
        showToast('info', 'duplicated that ledger cycle')
      } catch (err) {
        console.error('failed to duplicate ledger', err)
        showToast('error', "couldn't duplicate — try again")
      }
    },
    deleteLedgerEntries: async (ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deleteLedger(id)))
        setLedgers((prev) => prev.filter((l) => !ids.includes(l.id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that ledger cycle'
            : `deleted ${ids.length} ledger cycles`,
        )
      } catch (err) {
        console.error('failed to delete ledger', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    deleteLedgerBillEntries: async (ledgerId, ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deleteLedgerBill(ledgerId, id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that bill'
            : `deleted ${ids.length} bills`,
        )
      } catch (err) {
        console.error('failed to delete ledger bill', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    setActiveLedger: (id) => setActiveLedgerId(id),
    selectLedgerBill: (entry) => setSelectedLedgerBill(entry),
    addLedgerBillEntry: async (input) => {
      if (!activeLedgerId) return false
      try {
        await createLedgerBill(activeLedgerId, input)
        return true
      } catch (err) {
        console.error('failed to create ledger bill', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editLedgerBillEntry: async (input) => {
      if (!activeLedgerId || !selectedLedgerBill) return false
      try {
        await updateLedgerBill(activeLedgerId, selectedLedgerBill.id, input)
        return true
      } catch (err) {
        console.error('failed to update ledger bill', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    selectIncome: (id) => setIncomeSelected(id),
    addIncomeEntry: async (input) => {
      try {
        const created = await createIncome(input)
        setIncome((prev) => [created, ...prev])
        return true
      } catch (err) {
        console.error('failed to create income', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editIncomeEntry: async (input) => {
      if (!incomeSelected) return false
      try {
        const updated = await apiUpdateIncome(incomeSelected, input)
        setIncome((prev) =>
          prev.map((i) => (i.id === updated.id ? updated : i)),
        )
        return true
      } catch (err) {
        console.error('failed to update income', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    deleteIncomeEntries: async (ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => apiDeleteIncome(id)))
        setIncome((prev) => prev.filter((i) => !ids.includes(i.id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that income entry'
            : `deleted ${ids.length} income entries`,
        )
      } catch (err) {
        console.error('failed to delete income', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    duplicateIncomeEntry: async (id) => {
      const original = income.find((i) => i.id === id)
      if (!original) return
      try {
        const created = await createIncome({
          date: original.date,
          amount: original.amount,
          ledgerId: null,
          notes: original.notes,
        })
        setIncome((prev) => [created, ...prev])
        showToast('info', 'duplicated that income entry')
      } catch (err) {
        console.error('failed to duplicate income', err)
        showToast('error', "couldn't duplicate — try again")
      }
    },
    attachIncomesToLedger: async (ledgerId, ids) => {
      if (ids.length === 0) return
      try {
        const updated = await Promise.all(
          ids.map((id) => {
            const original = income.find((i) => i.id === id)
            if (!original) throw new Error(`income ${id} not found`)
            return apiUpdateIncome(id, {
              date: original.date,
              amount: original.amount,
              ledgerId,
              notes: original.notes,
            })
          }),
        )
        setIncome((prev) =>
          prev.map((i) => updated.find((u) => u.id === i.id) ?? i),
        )
        showToast(
          'info',
          ledgerId
            ? ids.length === 1
              ? 'attached that income entry'
              : `attached ${ids.length} income entries`
            : ids.length === 1
              ? 'removed that income from the ledger'
              : `removed ${ids.length} incomes from the ledger`,
        )
      } catch (err) {
        console.error('failed to attach income to ledger', err)
        showToast('error', "couldn't save that — try again")
      }
    },
    showToast,
    dismissToast,
    requestDelete,
    confirmDelete,
    cancelDelete,
    selectPtoYear: (id) => setPtoYearSelected(id),
    addPtoYearEntry: async (input) => {
      try {
        const created = await createPto(input)
        setPtos((prev) => [created, ...prev])
        return true
      } catch (err) {
        console.error('failed to create pto', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editPtoYearEntry: async (input) => {
      if (!ptoYearSelected) return false
      try {
        const updated = await updatePto(ptoYearSelected, input)
        setPtos((prev) => prev.map((p) => (p.id === updated.id ? updated : p)))
        return true
      } catch (err) {
        console.error('failed to update pto', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    duplicatePtoYearEntry: async (id) => {
      const original = ptos.find((p) => p.id === id)
      if (!original) return
      try {
        const created = await createPto({
          year: original.year,
          prevYearHours: original.prevYearHours,
          availableHours: original.availableHours,
          rolloverHours: original.rolloverHours,
        })
        setPtos((prev) => [created, ...prev])
        showToast('info', 'duplicated that PTO year')
      } catch (err) {
        console.error('failed to duplicate pto', err)
        showToast('error', "couldn't duplicate — try again")
      }
    },
    deletePtoYearEntries: async (ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deletePto(id)))
        setPtos((prev) => prev.filter((p) => !ids.includes(p.id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that PTO year'
            : `deleted ${ids.length} PTO years`,
        )
      } catch (err) {
        console.error('failed to delete pto', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    setActivePto: (id) => setActivePtoId(id),
    selectPtoPlan: (entry) => setSelectedPtoPlan(entry),
    addPtoPlanEntry: async (input) => {
      if (!activePtoId) return false
      try {
        await createPtoPlan(activePtoId, input)
        return true
      } catch (err) {
        console.error('failed to create pto plan', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editPtoPlanEntry: async (input) => {
      if (!activePtoId || !selectedPtoPlan) return false
      try {
        await updatePtoPlan(activePtoId, selectedPtoPlan.id, input)
        return true
      } catch (err) {
        console.error('failed to update pto plan', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    duplicatePtoPlanEntry: async (input) => {
      if (!activePtoId) return null
      try {
        const created = await createPtoPlan(activePtoId, input)
        showToast('info', 'duplicated that PTO entry')
        return created
      } catch (err) {
        console.error('failed to duplicate pto plan', err)
        showToast('error', "couldn't duplicate — try again")
        return null
      }
    },
    deletePtoPlanEntries: async (ptoId, ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deletePtoPlan(ptoId, id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that PTO entry'
            : `deleted ${ids.length} PTO entries`,
        )
      } catch (err) {
        console.error('failed to delete pto plan', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    selectHoliday: (entry) => setSelectedHoliday(entry),
    addHolidayEntry: async (input) => {
      if (!activePtoId) return false
      try {
        await createHolidayHour(activePtoId, input)
        return true
      } catch (err) {
        console.error('failed to create holiday hour', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    editHolidayEntry: async (input) => {
      if (!activePtoId || !selectedHoliday) return false
      try {
        await updateHolidayHour(activePtoId, selectedHoliday.id, input)
        return true
      } catch (err) {
        console.error('failed to update holiday hour', err)
        showToast('error', "couldn't save that — try again")
        return false
      }
    },
    duplicateHolidayEntry: async (input) => {
      if (!activePtoId) return null
      try {
        const created = await createHolidayHour(activePtoId, input)
        showToast('info', 'duplicated that holiday')
        return created
      } catch (err) {
        console.error('failed to duplicate holiday hour', err)
        showToast('error', "couldn't duplicate — try again")
        return null
      }
    },
    deleteHolidayEntries: async (ptoId, ids) => {
      if (ids.length === 0) return
      try {
        await Promise.all(ids.map((id) => deleteHolidayHour(ptoId, id)))
        showToast(
          'info',
          ids.length === 1
            ? 'deleted that holiday'
            : `deleted ${ids.length} holidays`,
        )
      } catch (err) {
        console.error('failed to delete holiday hour', err)
        showToast('error', "couldn't delete — try again")
      }
    },
    openBillModal: (mode) => {
      if (mode === 'Add') setBillSelected(null)
      setModal('bill')
      setModalMode(mode)
    },
    openIncomeModal: (mode) => {
      if (mode === 'Add') setIncomeSelected(null)
      setModal('income')
      setModalMode(mode)
    },
    openLedgerModal: (mode) => {
      if (mode === 'Add') setLedgerSelected(null)
      setModal('ledger')
      setModalMode(mode)
    },
    openLedgerBillModal: (mode) => {
      if (mode === 'Add') setSelectedLedgerBill(null)
      setModal('ledgerBill')
      setModalMode(mode)
    },
    openPtoYearModal: (mode) => {
      if (mode === 'Add') setPtoYearSelected(null)
      setModal('ptoYear')
      setModalMode(mode)
    },
    openPtoPlanModal: (mode) => {
      if (mode === 'Add') setSelectedPtoPlan(null)
      setModal('pto')
      setModalMode(mode)
    },
    openHolidayModal: (mode) => {
      if (mode === 'Add') setSelectedHoliday(null)
      setModal('holiday')
      setModalMode(mode)
    },
    closeModal: () => setModal(null),
  }

  return (
    <AppStoreContext.Provider value={store}>
      {children}
    </AppStoreContext.Provider>
  )
}

export function useAppStore(): AppStore {
  const store = useContext(AppStoreContext)
  if (!store)
    throw new Error('useAppStore must be used within AppStoreProvider')
  return store
}
