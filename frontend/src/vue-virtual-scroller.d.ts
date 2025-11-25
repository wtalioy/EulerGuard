declare module 'vue-virtual-scroller' {
  import type { DefineComponent } from 'vue'
  
  export const RecycleScroller: DefineComponent<{
    items: unknown[]
    itemSize?: number | null
    keyField?: string
    direction?: 'vertical' | 'horizontal'
    buffer?: number
  }>
  
  export const DynamicScroller: DefineComponent<{
    items: unknown[]
    minItemSize: number
    keyField?: string
  }>
  
  export const DynamicScrollerItem: DefineComponent<{
    item: unknown
    active: boolean
    sizeDependencies?: unknown[]
  }>
}

