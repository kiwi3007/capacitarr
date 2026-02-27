<template>
  <header class="border-b border-zinc-200 dark:border-zinc-800 bg-white/80 dark:bg-zinc-950/80 backdrop-blur-md sticky top-0 z-50">
    <UContainer>
      <div class="flex items-center justify-between h-16">
        <div class="flex items-center gap-6">
          <NuxtLink to="/" class="flex items-center gap-2">
            <UIcon name="i-heroicons-circle-stack" class="w-8 h-8 text-violet-500" />
            <span class="text-xl font-bold tracking-tight text-zinc-900 dark:text-white">Capacitarr</span>
          </NuxtLink>
          <nav class="hidden sm:flex items-center gap-1">
            <NuxtLink to="/" class="px-3 py-1.5 rounded-md text-sm font-medium transition-colors" :class="$route.path === '/' ? 'text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-950' : 'text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'">
              Dashboard
            </NuxtLink>
            <NuxtLink to="/settings" class="px-3 py-1.5 rounded-md text-sm font-medium transition-colors" :class="$route.path === '/settings' ? 'text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-950' : 'text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white'">
              Settings
            </NuxtLink>
          </nav>
        </div>
        
        <div class="flex items-center gap-4">
          <UButton variant="ghost" color="gray" icon="i-heroicons-moon" v-if="!isDark" @click="isDark = true" aria-label="Dark mode" />
          <UButton variant="ghost" color="gray" icon="i-heroicons-sun" v-else @click="isDark = false" aria-label="Light mode" />
          
          <UDropdown :items="userDropdownItems">
            <UAvatar src="https://avatars.githubusercontent.com/u/10?v=4" alt="Avatar" size="sm" />
          </UDropdown>
        </div>
      </div>
    </UContainer>
  </header>
</template>

<script setup lang="ts">
const colorMode = useColorMode()
const isDark = computed({
  get() {
    return colorMode.value === 'dark'
  },
  set() {
    colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
  }
})

const router = useRouter()
const token = useCookie('jwt')

const logout = () => {
  token.value = null
  router.push('/login')
}

const userDropdownItems = [
  [{
    label: 'Profile',
    icon: 'i-heroicons-user'
  }, {
    label: 'Settings',
    icon: 'i-heroicons-cog-8-tooth'
  }],
  [{
    label: 'Logout',
    icon: 'i-heroicons-arrow-right-on-rectangle',
    click: logout
  }]
]
</script>
