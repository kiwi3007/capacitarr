<template>
  <header
    data-slot="navbar"
    class="sticky top-0 z-50 relative"
  >
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16">
        <!-- Brand -->
        <div class="flex items-center gap-6">
          <NuxtLink
            to="/"
            class="flex items-center gap-2.5 group"
          >
            <div
              data-slot="brand-icon"
              class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center"
            >
              <component
                :is="DatabaseIcon"
                class="w-4.5 h-4.5 text-primary-foreground"
              />
            </div>
            <div class="flex flex-col">
              <span class="text-lg font-bold tracking-tight text-foreground leading-tight">
                Capacitarr
              </span>
              <span class="text-[10px] text-muted-foreground/50 leading-none font-mono">
                UI v{{ uiVersion }} · API {{ apiVersion || '···' }}
              </span>
              <span class="text-[10px] text-muted-foreground/40 leading-none italic mt-0.5">
                You paid for that disk, use it!
              </span>
            </div>
          </NuxtLink>

          <!-- Nav Links -->
          <nav
            aria-label="Main navigation"
            class="hidden sm:flex items-center gap-1"
          >
            <NuxtLink
              v-for="link in navLinks"
              :key="link.to"
              :to="link.to"
              class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all duration-200"
              :class="[
                $route.path === link.to
                  ? 'text-primary bg-primary/10'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              ]"
              :data-slot="$route.path === link.to ? 'nav-link-active' : undefined"
            >
              {{ link.label }}
            </NuxtLink>
          </nav>
        </div>

        <!-- Right side -->
        <div class="flex items-center gap-1">
          <!-- Engine Control -->
          <EngineControlPopover />

          <!-- Notification Bell -->
          <UiPopover @update:open="onNotifPopoverToggle">
            <UiPopoverTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="Notifications"
                class="relative"
              >
                <component
                  :is="BellIcon"
                  class="w-5 h-5"
                />
                <span
                  v-if="unreadCount > 0"
                  class="absolute -top-0.5 -right-0.5 flex items-center justify-center min-w-[18px] h-[18px] rounded-full bg-destructive text-destructive-foreground text-[10px] font-bold leading-none px-1"
                >
                  {{ unreadCount > 99 ? '99+' : unreadCount }}
                </span>
              </UiButton>
            </UiPopoverTrigger>
            <UiPopoverContent
              align="end"
              class="w-80 p-0"
            >
              <div class="flex items-center justify-between px-4 py-3 border-b border-border">
                <h4 class="text-sm font-semibold">
                  {{ $t('nav.notifications') }}
                </h4>
                <UiButton
                  v-if="unreadCount > 0"
                  variant="ghost"
                  size="sm"
                  class="h-auto py-1 px-2 text-xs"
                  @click="markAllAsRead"
                >
                  {{ $t('nav.markAllRead') }}
                </UiButton>
              </div>
              <UiScrollArea class="max-h-80">
                <div
                  v-if="notifLoading"
                  class="flex justify-center py-8"
                >
                  <component
                    :is="LoaderCircleIcon"
                    class="w-5 h-5 text-primary animate-spin"
                  />
                </div>
                <div
                  v-else-if="notifications.length === 0"
                  class="text-center py-8 text-sm text-muted-foreground"
                >
                  {{ $t('nav.noNotifications') }}
                </div>
                <div v-else>
                  <button
                    v-for="notif in notifications"
                    :key="notif.id"
                    class="flex items-start gap-3 w-full px-4 py-3 text-left transition-colors hover:bg-accent border-b border-border last:border-0"
                    :class="notif.read ? 'opacity-60' : ''"
                    @click="onNotifClick(notif)"
                  >
                    <component
                      :is="severityIcon(notif.severity)"
                      class="w-4 h-4 mt-0.5 shrink-0"
                      :class="severityColor(notif.severity)"
                    />
                    <div class="flex-1 min-w-0">
                      <p class="text-sm font-medium leading-tight truncate">
                        {{ notif.title }}
                      </p>
                      <p class="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                        {{ notif.message }}
                      </p>
                      <p class="text-[10px] text-muted-foreground/60 mt-1">
                        {{ formatRelativeTime(notif.createdAt) }}
                      </p>
                    </div>
                    <span
                      v-if="!notif.read"
                      class="w-2 h-2 rounded-full bg-primary shrink-0 mt-1.5"
                    />
                  </button>
                </div>
              </UiScrollArea>
            </UiPopoverContent>
          </UiPopover>

          <!-- Theme selector -->
          <UiDropdownMenu>
            <UiDropdownMenuTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="Change theme"
              >
                <component
                  :is="PaletteIcon"
                  class="w-5 h-5"
                />
              </UiButton>
            </UiDropdownMenuTrigger>
            <UiDropdownMenuContent
              align="end"
              class="w-40"
            >
              <UiDropdownMenuLabel>{{ $t('nav.theme') }}</UiDropdownMenuLabel>
              <UiDropdownMenuSeparator />
              <UiDropdownMenuItem
                v-for="t in themes"
                :key="t.id"
                class="flex items-center gap-2.5 cursor-pointer"
                @click="setTheme(t.id)"
              >
                <span
                  class="w-4 h-4 rounded-full border-2 shrink-0"
                  :class="theme === t.id ? 'border-primary' : 'border-transparent'"
                  :style="{ backgroundColor: themeSwatchColor(t) }"
                />
                <span>{{ t.label }}</span>
                <component
                  :is="CheckIcon"
                  v-if="theme === t.id"
                  class="w-4 h-4 ml-auto text-primary"
                />
              </UiDropdownMenuItem>
            </UiDropdownMenuContent>
          </UiDropdownMenu>

          <!-- Language selector -->
          <UiDropdownMenu>
            <UiDropdownMenuTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="Change language"
              >
                <component
                  :is="GlobeIcon"
                  class="w-5 h-5"
                />
              </UiButton>
            </UiDropdownMenuTrigger>
            <UiDropdownMenuContent
              align="end"
              class="w-40"
            >
              <UiDropdownMenuLabel>{{ $t('settings.language') }}</UiDropdownMenuLabel>
              <UiDropdownMenuSeparator />
              <UiDropdownMenuItem
                v-for="loc in locales"
                :key="typeof loc === 'string' ? loc : loc.code"
                class="flex items-center justify-between cursor-pointer"
                @click="setLocale(typeof loc === 'string' ? loc : loc.code)"
              >
                <span>{{ typeof loc === 'string' ? loc : loc.name }}</span>
                <component
                  :is="CheckIcon"
                  v-if="locale === (typeof loc === 'string' ? loc : loc.code)"
                  class="w-4 h-4 text-primary"
                />
              </UiDropdownMenuItem>
            </UiDropdownMenuContent>
          </UiDropdownMenu>

          <!-- Dark mode toggle -->
          <UiButton
            variant="ghost"
            size="icon"
            :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
            @click="toggle"
          >
            <component
              :is="isDark ? SunIcon : MoonIcon"
              class="w-5 h-5"
            />
          </UiButton>

          <!-- Help -->
          <UiButton
            variant="ghost"
            size="icon"
            as-child
          >
            <NuxtLink
              to="/help"
              aria-label="Help"
            >
              <component
                :is="CircleHelpIcon"
                class="w-5 h-5"
              />
            </NuxtLink>
          </UiButton>

          <!-- About -->
          <UiPopover>
            <UiPopoverTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="About Capacitarr"
              >
                <component
                  :is="InfoIcon"
                  class="w-5 h-5"
                />
              </UiButton>
            </UiPopoverTrigger>
            <UiPopoverContent
              align="end"
              class="w-72"
            >
              <div class="space-y-3">
                <div>
                  <h4 class="font-semibold text-sm">
                    Capacitarr
                  </h4>
                </div>
                <p class="text-sm text-muted-foreground leading-snug">
                  Automated media library capacity management — score, rank, and clean up your *arr libraries.
                </p>
                <UiSeparator />
                <div class="space-y-1.5 text-xs text-muted-foreground font-mono">
                  <div class="flex items-baseline justify-between gap-2">
                    <span>UI v{{ uiVersion }}</span>
                    <span
                      v-if="uiBuildDate"
                      class="text-muted-foreground/60"
                    >{{ formatBuildDate(uiBuildDate) }}</span>
                  </div>
                  <div class="flex items-baseline justify-between gap-2">
                    <span>API {{ apiVersion || '···' }}</span>
                    <span
                      v-if="apiBuildDate"
                      class="text-muted-foreground/60"
                    >{{ formatBuildDate(apiBuildDate) }}</span>
                  </div>
                </div>
                <UiSeparator />
                <div class="space-y-1.5 text-xs text-muted-foreground">
                  <p>Built by <span class="font-semibold text-foreground">Ghent Starshadow</span></p>
                  <p>
                    Inspired by
                    <a
                      href="https://github.com/jorenn92/Maintainerr"
                      target="_blank"
                      rel="noopener"
                      class="text-primary hover:underline inline-flex items-center gap-0.5"
                    >
                      Maintainerr <component
                        :is="ExternalLinkIcon"
                        class="w-3 h-3"
                      />
                    </a>
                    and the
                    <a
                      href="https://wiki.servarr.com/"
                      target="_blank"
                      rel="noopener"
                      class="text-primary hover:underline inline-flex items-center gap-0.5"
                    >
                      *arr <component
                        :is="ExternalLinkIcon"
                        class="w-3 h-3"
                      />
                    </a>
                    ecosystem
                  </p>
                </div>
                <UiSeparator />
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-muted-foreground/60">Go · Nuxt · shadcn-vue · SQLite</span>
                  <a
                    href="https://gitlab.com/starshadow/software/capacitarr"
                    target="_blank"
                    rel="noopener"
                    class="text-xs text-primary hover:underline inline-flex items-center gap-1"
                  >
                    GitLab <component
                      :is="ExternalLinkIcon"
                      class="w-3 h-3"
                    />
                  </a>
                </div>
              </div>
            </UiPopoverContent>
          </UiPopover>

          <!-- Logout -->
          <UiButton
            variant="ghost"
            size="icon"
            aria-label="Logout"
            class="hover:text-destructive"
            @click="logout"
          >
            <component
              :is="LogOutIcon"
              class="w-5 h-5"
            />
          </UiButton>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import {
  DatabaseIcon, MoonIcon, SunIcon, LogOutIcon, CircleHelpIcon, PaletteIcon,
  CheckIcon, InfoIcon, ExternalLinkIcon, BellIcon, LoaderCircleIcon,
  AlertTriangleIcon, XCircleIcon, CheckCircleIcon, GlobeIcon
} from 'lucide-vue-next'
import { formatRelativeTime } from '~/utils/format'
import type { ThemeMeta } from '~/composables/useTheme'
import type { InAppNotification } from '~/types/api'

const { isDark, toggle } = useAppColorMode()
const { theme, setTheme, themes } = useTheme()
const { uiVersion, uiBuildDate, apiVersion, apiBuildDate } = useVersion()
const {
  unreadCount,
  notifications,
  loading: notifLoading,
  fetchNotifications,
  markAsRead,
  markAllAsRead,
  startPolling,
  stopPolling
} = useNotifications()

/** Map severity to icon component */
function severityIcon(severity: string) {
  switch (severity) {
    case 'info': return InfoIcon
    case 'warning': return AlertTriangleIcon
    case 'error': return XCircleIcon
    case 'success': return CheckCircleIcon
    default: return InfoIcon
  }
}

/** Map severity to text color class */
function severityColor(severity: string) {
  switch (severity) {
    case 'info': return 'text-blue-500'
    case 'warning': return 'text-amber-500'
    case 'error': return 'text-red-500'
    case 'success': return 'text-green-500'
    default: return 'text-muted-foreground'
  }
}

/** When the popover opens, fetch notifications; when it closes, do nothing */
function onNotifPopoverToggle(open: boolean) {
  if (open) {
    fetchNotifications()
  }
}

/** Click a notification to mark it as read */
function onNotifClick(notif: InAppNotification) {
  if (!notif.read) {
    markAsRead(notif.id)
  }
}

onMounted(() => {
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})

/** Format an ISO date string as a short readable date */
function formatBuildDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}
const router = useRouter()
const authenticated = useCookie('authenticated')

const { t, locale, locales, setLocale } = useI18n()

const navLinks = computed(() => [
  { to: '/', label: t('nav.dashboard') },
  { to: '/rules', label: t('nav.scoringEngine') },
  { to: '/audit', label: t('nav.auditLog') },
  { to: '/settings', label: t('nav.settings') }
])

function logout() {
  authenticated.value = null
  router.push('/login')
}

/** Map theme hue to a swatch color for the dropdown */
function themeSwatchColor(t: ThemeMeta): string {
  return `oklch(0.6 0.2 ${t.hue})`
}
</script>
