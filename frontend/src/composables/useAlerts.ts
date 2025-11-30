import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getAlerts, subscribeToAlerts, type Alert } from '../lib/api'

export { Alert }

export function useAlerts() {
  const alerts = ref<Alert[]>([])
  const newAlertCount = ref(0)

  // Cumulative severity counters
  const severityCounts = ref({
    critical: 0,
    high: 0,
    warning: 0,
    info: 0
  })

  // Action counters
  const actionCounts = ref({
    blocked: 0,
    alerted: 0
  })

  let unsubscribe: (() => void) | null = null
  let lastAlertIds = new Set<string>()

  const updateCounts = (alertList: Alert[]) => {
    severityCounts.value = {
      critical: 0,
      high: 0,
      warning: 0,
      info: 0
    }
    actionCounts.value = {
      blocked: 0,
      alerted: 0
    }

    for (const alert of alertList) {
      // Count by severity
      switch (alert.severity) {
        case 'critical':
          severityCounts.value.critical++
          break
        case 'high':
          severityCounts.value.high++
          break
        case 'warning':
          severityCounts.value.warning++
          break
        case 'info':
          severityCounts.value.info++
          break
      }

      // Count by action
      if (alert.blocked) {
        actionCounts.value.blocked++
      } else {
        actionCounts.value.alerted++
      }
    }
  }

  const fetchAlerts = async () => {
    try {
      const result = await getAlerts()
      alerts.value = result || []
      updateCounts(alerts.value)
    } catch (e) {
      console.error('Failed to fetch alerts:', e)
    }
  }

  const handleAlertsUpdate = (newAlerts: Alert[]) => {
    const newIds = new Set(newAlerts.map(a => a.id))
    const addedAlerts = newAlerts.filter(a => !lastAlertIds.has(a.id))

    if (addedAlerts.length > 0) {
      newAlertCount.value += addedAlerts.length
    }

    alerts.value = newAlerts.slice(0, 100)
    updateCounts(alerts.value)
    lastAlertIds = newIds
  }

  const clearNewCount = () => {
    newAlertCount.value = 0
  }

  const getAlertsBySeverity = () => {
    return {
      critical: severityCounts.value.critical,
      high: severityCounts.value.high,
      warning: severityCounts.value.warning,
      info: severityCounts.value.info
    }
  }

  const getAlertsByAction = () => {
    return {
      blocked: actionCounts.value.blocked,
      alerted: actionCounts.value.alerted
    }
  }

  // Computed for quick access to blocked alerts
  const blockedAlerts = computed(() => 
    alerts.value.filter(a => a.blocked)
  )

  const alertedOnly = computed(() => 
    alerts.value.filter(a => !a.blocked)
  )

  onMounted(async () => {
    await fetchAlerts()
    lastAlertIds = new Set(alerts.value.map(a => a.id))
    unsubscribe = subscribeToAlerts(handleAlertsUpdate)
  })

  onUnmounted(() => {
    unsubscribe?.()
  })

  return {
    alerts,
    blockedAlerts,
    alertedOnly,
    newAlertCount,
    clearNewCount,
    fetchAlerts,
    getAlertsBySeverity,
    getAlertsByAction
  }
}
