<template>
  <div class="flex items-center justify-center min-h-[calc(100vh-100px)]">
    <UCard class="w-full max-w-sm">
      <template #header>
        <div class="text-center">
          <UIcon name="i-heroicons-lock-closed" class="w-12 h-12 text-violet-500 mb-4 mx-auto" />
          <h2 class="text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">Welcome Back</h2>
          <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1">Please sign in to Capacitarr</p>
        </div>
      </template>

      <UForm :state="state" class="space-y-4" @submit="onSubmit">
        <UFormGroup label="Username" name="username">
          <UInput v-model="state.username" placeholder="admin" autofocus />
        </UFormGroup>

        <UFormGroup label="Password" name="password">
          <UInput v-model="state.password" type="password" placeholder="••••••••" />
        </UFormGroup>

        <UButton type="submit" color="primary" block :loading="loading">
          Sign In
        </UButton>
      </UForm>

      <template #footer v-if="errorMsg">
        <div class="text-sm text-red-500 text-center">{{ errorMsg }}</div>
      </template>
    </UCard>
  </div>
</template>

<script setup lang="ts">
import { ofetch } from 'ofetch'

const config = useRuntimeConfig()
const router = useRouter()
const token = useCookie('jwt')

const state = reactive({
  username: '',
  password: ''
})

const loading = ref(false)
const errorMsg = ref('')

// If already authenticated, redirect
if (token.value) {
  router.push('/')
}

async function onSubmit(event: any) {
  errorMsg.value = ''
  loading.value = true

  try {
    const response = await ofetch(`${config.public.apiBaseUrl}/api/v1/auth/login`, {
      method: 'POST',
      body: {
        username: state.username,
        password: state.password
      }
    })

    if (response.message === 'success') {
      // Set the JWT cookie from response so useCookie can read it
      if (response.token) {
        token.value = response.token
      }
      // Full page reload to ensure all components pick up the auth state
      window.location.href = '/'
    } else {
       errorMsg.value = 'Authentication failed'
    }
  } catch (e: any) {
    if (e.response?.status === 401) {
      errorMsg.value = 'Invalid username or password'
    } else {
      errorMsg.value = 'Network error connecting to backend'
    }
  } finally {
    loading.value = false
  }
}
</script>
