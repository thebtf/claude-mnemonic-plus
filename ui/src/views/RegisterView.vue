<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2, AlertCircle } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'

const router = useRouter()
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const invitationCode = ref('')
const error = ref('')
const submitting = ref(false)

async function handleRegister() {
  error.value = ''
  if (!email.value.trim() || !password.value || !invitationCode.value.trim()) {
    error.value = 'All fields are required.'
    return
  }
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match.'
    return
  }
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters.'
    return
  }

  submitting.value = true
  try {
    const resp = await fetch('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: email.value.trim(),
        password: password.value,
        invitation: invitationCode.value.trim(),
      }),
    })
    if (!resp.ok) {
      const data = await resp.json().catch(() => ({}))
      error.value = data.error || 'Registration failed.'
      return
    }
    router.push({ name: 'login' })
  } catch {
    error.value = 'Connection error.'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-background px-4">
    <div class="w-full max-w-md">
      <Card>
        <CardHeader class="text-center pb-4">
          <div class="w-14 h-14 mx-auto mb-4 rounded-2xl bg-primary flex items-center justify-center">
            <span class="text-primary-foreground font-bold text-xl">E</span>
          </div>
          <CardTitle class="text-2xl">Create account</CardTitle>
          <CardDescription>Register with an invitation code</CardDescription>
        </CardHeader>

        <CardContent>
          <form @submit.prevent="handleRegister" class="space-y-4">
            <div class="space-y-2">
              <Label for="reg-email">Email</Label>
              <Input
                id="reg-email"
                v-model="email"
                type="email"
                autocomplete="email"
                placeholder="you@example.com"
                :disabled="submitting"
              />
            </div>

            <div class="space-y-2">
              <Label for="reg-password">Password</Label>
              <Input
                id="reg-password"
                v-model="password"
                type="password"
                autocomplete="new-password"
                placeholder="Min 8 characters"
                :disabled="submitting"
              />
            </div>

            <div class="space-y-2">
              <Label for="reg-confirm-password">Confirm Password</Label>
              <Input
                id="reg-confirm-password"
                v-model="confirmPassword"
                type="password"
                autocomplete="new-password"
                :disabled="submitting"
              />
            </div>

            <div class="space-y-2">
              <Label for="reg-invitation">Invitation Code</Label>
              <Input
                id="reg-invitation"
                v-model="invitationCode"
                type="text"
                autocomplete="off"
                placeholder="Paste your invitation code"
                class="font-mono text-sm"
                :disabled="submitting"
              />
            </div>

            <p v-if="error" class="flex items-center gap-2 text-sm text-destructive">
              <AlertCircle class="w-4 h-4 shrink-0" />
              {{ error }}
            </p>

            <Button type="submit" class="w-full" :disabled="submitting">
              <Loader2 v-if="submitting" class="w-4 h-4 mr-2 animate-spin" />
              {{ submitting ? 'Creating account...' : 'Create Account' }}
            </Button>

            <p class="text-center text-sm text-muted-foreground">
              Already have an account?
              <router-link
                :to="{ name: 'login' }"
                class="text-primary hover:text-primary/80 font-medium transition-colors"
              >
                Sign in
              </router-link>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
