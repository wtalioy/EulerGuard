import { ref, onMounted, onUnmounted } from 'vue'
import { getAlerts, subscribeToAlerts, type Alert } from '../lib/api'

export { Alert }

export function useAlerts() {
  const alerts = ref<Alert[]>([])
  const newAlertCount = ref(0)

  // Cumulative severity counters
  const severityCounts = ref({
    high: 0,
    warning: 0,
    info: 0
  })

  let unsubscribe: (() => void) | null = null
  let lastAlertIds = new Set<string>()

  const fetchAlerts = async () => {
    try {
      const result = await getAlerts()
      alerts.value = result || []

      // Initialize severity counts based on existing alerts
      severityCounts.value = {
        high: 0,
        warning: 0,
        info: 0
      }
      for (const alert of alerts.value) {
        switch (alert.severity) {
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
      }
    } catch (e) {
      console.error('Failed to fetch alerts:', e)
    }
  }

  const handleAlertsUpdate = (newAlerts: Alert[]) => {
    const newIds = new Set(newAlerts.map(a => a.id))
    const addedAlerts = newAlerts.filter(a => !lastAlertIds.has(a.id))

    if (addedAlerts.length > 0) {
      newAlertCount.value += addedAlerts.length

      // Update cumulative severity counts
      for (const alert of addedAlerts) {
        switch (alert.severity) {
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
      }
    }

    alerts.value = newAlerts.slice(0, 100)
    lastAlertIds = newIds
  }

  const clearNewCount = () => {
    newAlertCount.value = 0
  }

  const getAlertsBySeverity = () => {
    return {
      high: severityCounts.value.high,
      warning: severityCounts.value.warning,
      info: severityCounts.value.info
    }
  }

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
    newAlertCount,
    clearNewCount,
    fetchAlerts,
    getAlertsBySeverity
  }
}
