<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { Close, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

const open = defineModel('open', { type: Boolean, default: false })
const mode = defineModel('mode', { type: String, default: 'login' })
const props = defineProps({
  allowRegistration: { type: Boolean, default: true },
  emailWhitelist: { type: [String, Array], default: '[]' }
})
const auth = useAuthStore()
const form = reactive({ username: '', email: '', password: '', emailCode: '' })
const captcha = reactive({ challengeId: '', image: '', trackWidth: 280, pieceWidth: 42, x: 0 })
const loading = ref(false)
const sendingCode = ref(false)
const emailCodeCooldown = ref(0)
const securityOpen = ref(false)
const securityBusy = ref(false)
const pendingAction = ref('')
const error = ref('')
const notice = ref('')
let captchaTimer = null
let emailCodeTimer = null

const captchaPassed = computed(() => securityBusy.value)
const sliderMax = computed(() => Math.max(0, captcha.trackWidth - captcha.pieceWidth))
const pieceStyle = computed(() => ({
  left: `${(captcha.x / captcha.trackWidth) * 100}%`,
  width: `${(captcha.pieceWidth / captcha.trackWidth) * 100}%`
}))
const slideButtonStyle = computed(() => {
  const progress = sliderMax.value ? captcha.x / sliderMax.value : 0
  return { left: `calc(${progress * 100}% - ${progress * 56}px)` }
})
const captchaX = computed(() => Math.round(Math.min(captcha.trackWidth, Math.max(0, Number(captcha.x) || 0))))
const allowedEmailDomains = computed(() => parseEmailWhitelist(props.emailWhitelist))
const emailWhitelistTip = computed(() => {
  if (mode.value !== 'register' || allowedEmailDomains.value.length === 0) return ''
  return `请使用 ${allowedEmailDomains.value.map((item) => `@${item}`).join('、')} 后缀邮箱注册`
})

watch([open, mode], () => {
  const registrationBlocked = mode.value === 'register' && !props.allowRegistration
  error.value = registrationBlocked ? '当前站点暂未开放新用户注册' : ''
  notice.value = ''
  if (registrationBlocked) {
    mode.value = 'login'
  }
  closeSecurity()
})

watch(error, (message) => {
  if (message) ElMessage.error(message)
})

watch(notice, (message) => {
  if (message) ElMessage.success(message)
})

onBeforeUnmount(() => {
  clearEmailCodeCooldown()
  if (captchaTimer) clearTimeout(captchaTimer)
})

function switchMode(nextMode) {
  if (mode.value === nextMode) return
  if (nextMode === 'register' && !props.allowRegistration) {
    error.value = '当前站点暂未开放新用户注册'
    notice.value = ''
    return
  }
  mode.value = nextMode
  Object.assign(form, { username: '', email: '', password: '', emailCode: '' })
  error.value = ''
  notice.value = ''
  closeSecurity()
}

function clearEmailCodeCooldown() {
  if (emailCodeTimer) {
    clearInterval(emailCodeTimer)
    emailCodeTimer = null
  }
}

function startEmailCodeCooldown() {
  clearEmailCodeCooldown()
  emailCodeCooldown.value = 60
  emailCodeTimer = setInterval(() => {
    if (emailCodeCooldown.value <= 1) {
      emailCodeCooldown.value = 0
      clearEmailCodeCooldown()
      return
    }
    emailCodeCooldown.value -= 1
  }, 1000)
}

async function loadCaptcha() {
  const res = await api.post('/captcha/slide')
  captcha.challengeId = res.data.challenge_id
  captcha.image = res.data.image
  captcha.trackWidth = res.data.track_width
  captcha.pieceWidth = res.data.piece_width
  captcha.x = 0
}

function handleCaptchaSlide(event) {
  if (securityBusy.value) return
  captcha.x = Number(event.target.value || 0)
  if (captchaTimer) clearTimeout(captchaTimer)
  captchaTimer = setTimeout(() => {
    if (!securityOpen.value || securityBusy.value) return
    securityBusy.value = true
    finishSecurity()
  }, 360)
}

async function requestSecurity(action) {
  error.value = ''
  notice.value = ''
  pendingAction.value = action
  securityBusy.value = false
  securityOpen.value = true
  try {
    await loadCaptcha()
  } catch (err) {
    securityOpen.value = false
    error.value = err.message
  }
}

function closeSecurity() {
  if (captchaTimer) clearTimeout(captchaTimer)
  securityOpen.value = false
  securityBusy.value = false
  pendingAction.value = ''
  captcha.x = 0
}

async function finishSecurity() {
  const action = pendingAction.value
  securityOpen.value = false
  pendingAction.value = ''
  if (action === 'email') {
    await sendEmailCodeWithCaptcha()
    return
  }
  if (action === 'submit') {
    await submitWithCaptcha()
  }
}

function sendEmailCode() {
  if (!props.allowRegistration) {
    error.value = '当前站点暂未开放新用户注册'
    return
  }
  if (!form.email) {
    error.value = '请先填写邮箱'
    return
  }
  if (!emailAllowedByWhitelist(form.email)) {
    error.value = emailWhitelistTip.value || '请更换为白名单后缀邮箱'
    return
  }
  if (sendingCode.value || emailCodeCooldown.value > 0) return
  requestSecurity('email')
}

async function sendEmailCodeWithCaptcha() {
  sendingCode.value = true
  try {
    await api.post('/auth/email-code', {
      email: form.email,
      challenge_id: captcha.challengeId,
      captcha_x: captchaX.value
    })
    notice.value = '验证码已发送，请查收邮箱'
    startEmailCodeCooldown()
  } catch (err) {
    error.value = err.message
  } finally {
    sendingCode.value = false
    securityBusy.value = false
  }
}

function submit() {
  if (mode.value === 'register' && !props.allowRegistration) {
    error.value = '当前站点暂未开放新用户注册'
    return
  }
  if (mode.value === 'register' && !emailAllowedByWhitelist(form.email)) {
    error.value = emailWhitelistTip.value || '请更换为白名单后缀邮箱'
    return
  }
  requestSecurity('submit')
}

async function submitWithCaptcha() {
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login({
        email: form.email,
        password: form.password,
        challenge_id: captcha.challengeId,
        captcha_x: captchaX.value
      })
      open.value = false
    } else {
      await auth.register({
        username: form.username,
        email: form.email,
        password: form.password,
        email_code: form.emailCode,
        challenge_id: captcha.challengeId,
        captcha_x: captchaX.value
      })
      notice.value = '注册成功，邮箱已验证，请登录'
      mode.value = 'login'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
    securityBusy.value = false
  }
}

function parseEmailWhitelist(value) {
  const raw = Array.isArray(value) ? value : safeParseWhitelist(value)
  const seen = new Set()
  return raw
    .map((item) => String(item || '').trim().toLowerCase().replace(/^@/, ''))
    .filter((item) => {
      if (!item || !item.includes('.') || item.includes('@') || item.includes('/') || item.includes('\\') || item.startsWith('.') || item.endsWith('.') || seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function safeParseWhitelist(value) {
  try {
    return JSON.parse(value || '[]')
  } catch {
    return String(value || '').split(/[\s,;]+/)
  }
}

function emailAllowedByWhitelist(email) {
  if (mode.value !== 'register' || allowedEmailDomains.value.length === 0) return true
  const domain = String(email || '').trim().toLowerCase().split('@').pop()
  return allowedEmailDomains.value.includes(domain)
}
</script>

<template>
  <el-dialog v-model="open" class="auth-el-dialog" modal-class="auth-overlay" width="min(460px, calc(100vw - 32px))" align-center :show-close="false">
      <div class="auth-card">
        <div class="modal-head">
          <div>
            <p class="section-kicker">{{ mode === 'login' ? 'Welcome back' : 'Create account' }}</p>
            <h2>{{ mode === 'login' ? '登录' : '注册' }}</h2>
          </div>
          <el-button class="icon-button" :icon="Close" circle @click="open = false" />
        </div>

        <el-segmented
          v-if="allowRegistration"
          class="auth-tabs"
          :model-value="mode"
          :options="[
            { label: '登录', value: 'login' },
            { label: '注册', value: 'register' }
          ]"
          @update:model-value="switchMode"
        />

        <el-form class="space-y-4" label-position="top" @submit.prevent="submit">
          <el-form-item v-if="mode === 'register'" label="用户名" required>
            <el-input v-model="form.username" />
          </el-form-item>
          <el-form-item label="邮箱" required>
            <el-input v-model="form.email" type="email" />
          </el-form-item>
          <p v-if="emailWhitelistTip" class="auth-field-tip">{{ emailWhitelistTip }}</p>
          <el-form-item label="密码" required>
            <el-input v-model="form.password" type="password" minlength="8" show-password />
          </el-form-item>

          <el-form-item v-if="mode === 'register'" label="邮箱验证码" required>
            <el-input v-model="form.emailCode" maxlength="6">
              <template #append>
                <el-button type="primary" plain :disabled="sendingCode || emailCodeCooldown > 0" @click="sendEmailCode">
                {{ sendingCode ? '发送中' : emailCodeCooldown > 0 ? `${emailCodeCooldown}秒后重试` : '发送' }}
                </el-button>
              </template>
            </el-input>
          </el-form-item>

          <el-button class="w-full justify-center py-3" type="primary" native-type="submit" :loading="loading">
            {{ loading ? '处理中' : mode === 'login' ? '登录' : '创建账号' }}
          </el-button>
        </el-form>
      </div>

      <el-dialog v-model="securityOpen" class="security-el-dialog" modal-class="security-overlay" width="min(430px, calc(100vw - 32px))" append-to-body align-center :show-close="false">
          <section class="security-card security-dialog" :class="{ passed: captchaPassed }">
            <div class="security-head">
              <h3>请完成安全验证</h3>
              <el-button class="security-close" :icon="Close" circle @click="closeSecurity" />
            </div>
            <div class="captcha-stage">
              <div class="captcha-sky">
                <el-button class="security-refresh" :icon="Refresh" circle title="刷新验证码" @click="loadCaptcha" />
                <span class="planet one"></span>
                <span class="planet two"></span>
                <span class="planet three"></span>
                <span class="planet four"></span>
                <span class="trace trace-one"></span>
                <span class="trace trace-two"></span>
                <span class="trace trace-three"></span>
                <img v-if="captcha.image" class="captcha-image" :src="captcha.image" alt="" />
                <span class="captcha-fragment" :style="pieceStyle"></span>
              </div>
            </div>
            <div class="slide-rail" :style="{ '--slide-progress': `${sliderMax ? (captcha.x / sliderMax) * 100 : 0}%` }">
              <input
                :value="captcha.x"
                type="range"
                min="0"
                :max="sliderMax"
                aria-label="拖动滑块完成验证"
                @input="handleCaptchaSlide"
              />
              <span class="slide-button" :style="slideButtonStyle">›</span>
            </div>
            <p class="security-tip">{{ captchaPassed ? '验证通过，正在继续操作' : '向右拖动滑块完成验证' }}</p>
          </section>
      </el-dialog>
  </el-dialog>
</template>
