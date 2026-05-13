<script setup>
import { reactive, ref } from 'vue'
import { useAuthStore } from '../stores/auth'

const open = defineModel('open', { type: Boolean, default: false })
const mode = defineModel('mode', { type: String, default: 'login' })
const auth = useAuthStore()
const form = reactive({ username: '', email: '', password: '' })
const loading = ref(false)
const error = ref('')
const notice = ref('')

async function submit() {
  error.value = ''
  notice.value = ''
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login({ email: form.email, password: form.password })
      open.value = false
    } else {
      await auth.register({ username: form.username, email: form.email, password: form.password })
      notice.value = '注册成功，账号进入待审核状态'
      mode.value = 'login'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 grid place-items-center bg-black/70 px-4">
    <div class="w-full max-w-md rounded border border-line bg-panel p-6 shadow-2xl">
      <div class="mb-5 flex items-center justify-between">
        <h2 class="text-xl font-semibold">{{ mode === 'login' ? '登录' : '注册' }}</h2>
        <button class="focus-ring rounded px-2 py-1 text-slate-400" @click="open = false">×</button>
      </div>

      <div class="mb-5 grid grid-cols-2 rounded border border-line p-1">
        <button
          class="focus-ring rounded px-3 py-2 text-sm"
          :class="mode === 'login' ? 'bg-brand text-ink' : 'text-slate-300'"
          @click="mode = 'login'"
        >
          登录
        </button>
        <button
          class="focus-ring rounded px-3 py-2 text-sm"
          :class="mode === 'register' ? 'bg-brand text-ink' : 'text-slate-300'"
          @click="mode = 'register'"
        >
          注册
        </button>
      </div>

      <form class="space-y-4" @submit.prevent="submit">
        <label v-if="mode === 'register'" class="block">
          <span class="text-sm text-slate-300">用户名</span>
          <input v-model="form.username" class="focus-ring mt-1 w-full rounded border border-line bg-ink px-3 py-2" required />
        </label>
        <label class="block">
          <span class="text-sm text-slate-300">邮箱</span>
          <input v-model="form.email" class="focus-ring mt-1 w-full rounded border border-line bg-ink px-3 py-2" type="email" required />
        </label>
        <label class="block">
          <span class="text-sm text-slate-300">密码</span>
          <input v-model="form.password" class="focus-ring mt-1 w-full rounded border border-line bg-ink px-3 py-2" type="password" minlength="8" required />
        </label>
        <p v-if="error" class="text-sm text-red-300">{{ error }}</p>
        <p v-if="notice" class="text-sm text-brand">{{ notice }}</p>
        <button class="focus-ring w-full rounded bg-brand px-4 py-3 font-semibold text-ink" :disabled="loading">
          {{ loading ? '处理中' : mode === 'login' ? '登录' : '创建账号' }}
        </button>
      </form>
    </div>
  </div>
</template>
