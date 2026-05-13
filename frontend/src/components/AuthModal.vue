<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const open = defineModel('open', { type: Boolean, default: false })
const mode = defineModel('mode', { type: String, default: 'login' })
const auth = useAuthStore()
const form = reactive({ username: '', email: '', password: '', emailCode: '' })
const captcha = reactive({ challengeId: '', targetX: 0, trackWidth: 280, pieceWidth: 42, x: 0 })
const loading = ref(false)
const sendingCode = ref(false)
const error = ref('')
const notice = ref('')

const captchaReady = computed(() => Boolean(captcha.challengeId))
const captchaPassed = computed(() => Math.abs(captcha.x - captcha.targetX) <= 6)
const sliderMax = computed(() => Math.max(0, captcha.trackWidth - captcha.pieceWidth))

watch([open, mode], async () => {
  error.value = ''
  notice.value = ''
  if (open.value) await loadCaptcha()
})

async function loadCaptcha() {
  const res = await api.post('/captcha/slide')
  captcha.challengeId = res.data.challenge_id
  captcha.targetX = res.data.target_x
  captcha.trackWidth = res.data.track_width
  captcha.pieceWidth = res.data.piece_width
  captcha.x = 0
}

async function sendEmailCode() {
  if (!form.email) {
    error.value = '请先填写邮箱'
    return
  }
  if (!captchaPassed.value) {
    error.value = '请先完成滑动验证码'
    return
  }
  error.value = ''
  notice.value = ''
  sendingCode.value = true
  try {
    await api.post('/auth/email-code', {
      email: form.email,
      challenge_id: captcha.challengeId,
      captcha_x: Number(captcha.x)
    })
    notice.value = '验证码已发送，请查收邮箱'
    await loadCaptcha()
  } catch (err) {
    error.value = err.message
    await loadCaptcha()
  } finally {
    sendingCode.value = false
  }
}

async function submit() {
  error.value = ''
  notice.value = ''
  if (!captchaReady.value || !captchaPassed.value) {
    error.value = '请完成滑动验证码'
    return
  }
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login({
        email: form.email,
        password: form.password,
        challenge_id: captcha.challengeId,
        captcha_x: Number(captcha.x)
      })
      open.value = false
    } else {
      await auth.register({
        username: form.username,
        email: form.email,
        password: form.password,
        email_code: form.emailCode,
        challenge_id: captcha.challengeId,
        captcha_x: Number(captcha.x)
      })
      notice.value = '注册成功，账号进入待审核状态'
      mode.value = 'login'
      await loadCaptcha()
    }
  } catch (err) {
    error.value = err.message
    await loadCaptcha()
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 grid place-items-center bg-forest/70 px-4">
    <div class="w-full max-w-md rounded border border-line bg-panel p-6 shadow-panel">
      <div class="mb-5 flex items-center justify-between">
        <h2 class="text-xl font-black text-forest">{{ mode === 'login' ? '登录' : '注册' }}</h2>
        <button class="focus-ring rounded px-2 py-1 text-muted" @click="open = false">×</button>
      </div>

      <div class="mb-5 grid grid-cols-2 rounded border border-line p-1">
        <button
          class="focus-ring rounded px-3 py-2 text-sm"
          :class="mode === 'login' ? 'bg-brand text-white' : 'text-muted'"
          @click="mode = 'login'"
        >
          登录
        </button>
        <button
          class="focus-ring rounded px-3 py-2 text-sm"
          :class="mode === 'register' ? 'bg-brand text-white' : 'text-muted'"
          @click="mode = 'register'"
        >
          注册
        </button>
      </div>

      <form class="space-y-4" @submit.prevent="submit">
        <label v-if="mode === 'register'" class="block">
          <span class="text-sm font-semibold text-muted">用户名</span>
          <input v-model="form.username" class="focus-ring mt-1 w-full rounded border border-line bg-white px-3 py-2 text-forest" required />
        </label>
        <label class="block">
          <span class="text-sm font-semibold text-muted">邮箱</span>
          <input v-model="form.email" class="focus-ring mt-1 w-full rounded border border-line bg-white px-3 py-2 text-forest" type="email" required />
        </label>
        <label class="block">
          <span class="text-sm font-semibold text-muted">密码</span>
          <input v-model="form.password" class="focus-ring mt-1 w-full rounded border border-line bg-white px-3 py-2 text-forest" type="password" minlength="8" required />
        </label>

        <label v-if="mode === 'register'" class="block">
          <span class="text-sm font-semibold text-muted">邮箱验证码</span>
          <div class="mt-1 flex gap-2">
            <input v-model="form.emailCode" class="focus-ring min-w-0 flex-1 rounded border border-line bg-white px-3 py-2 text-forest" maxlength="6" required />
            <button class="focus-ring rounded border border-line bg-white px-3 py-2 text-sm font-bold" type="button" :disabled="sendingCode" @click="sendEmailCode">
              {{ sendingCode ? '发送中' : '发送' }}
            </button>
          </div>
        </label>

        <div class="rounded border border-line bg-mint p-3">
          <div class="mb-2 flex items-center justify-between text-xs font-semibold text-muted">
            <span>滑动验证码</span>
            <button class="focus-ring rounded px-2 py-1" type="button" @click="loadCaptcha">刷新</button>
          </div>
          <div class="relative h-10 rounded bg-white">
            <div class="absolute top-0 h-10 rounded border border-dashed border-accent bg-accent/10" :style="{ left: `${captcha.targetX}px`, width: `${captcha.pieceWidth}px` }"></div>
            <div class="absolute top-1 grid h-8 w-10 place-items-center rounded bg-brand text-xs font-black text-white shadow-sm" :style="{ left: `${captcha.x}px` }">
              ≡
            </div>
          </div>
          <input v-model.number="captcha.x" class="mt-3 w-full accent-[#169b7b]" type="range" min="0" :max="sliderMax" />
          <div class="mt-1 text-xs font-semibold" :class="captchaPassed ? 'text-brand' : 'text-muted'">
            {{ captchaPassed ? '验证通过' : '拖动滑块到缺口位置' }}
          </div>
        </div>

        <p v-if="error" class="text-sm text-red-700">{{ error }}</p>
        <p v-if="notice" class="text-sm font-bold text-brand">{{ notice }}</p>
        <button class="focus-ring w-full rounded bg-accent px-4 py-3 font-bold text-white" :disabled="loading">
          {{ loading ? '处理中' : mode === 'login' ? '登录' : '创建账号' }}
        </button>
      </form>
    </div>
  </div>
</template>
