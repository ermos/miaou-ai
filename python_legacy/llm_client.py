import requests
from config import LLM_URL, OPENAI_API_KEY, OPENAI_MODEL, LLM_TIMEOUT, LLM_TEMPERATURE
from personality_loader import PersonalityConfig


class ContextualLLMClient:
    def __init__(self, context_manager):
        self.api_url = f"{LLM_URL}/chat/completions"
        self.context_manager = context_manager
        self.personality = PersonalityConfig()

        if not OPENAI_API_KEY:
            print("⚠️  OPENAI_API_KEY is not set (check your .env)")

        print(f"🤖 LLM Client connected to {LLM_URL}")
        print(f"   Model: {OPENAI_MODEL}")
    
    def chat(self, user_text):
        """Chat avec contexte + personnalité"""
        
        # 1. Build personality-based system prompt
        system_prompt = self._build_system_prompt()
        
        # 2. Get context from memory
        context = self.context_manager.get_context_for_llm()
        
        # 3. Detect if correction needed
        correction_hint = self._detect_corrections(user_text)
        
        # 4. Combine everything
        user_prompt = f"""{context}

User message: {user_text}
{correction_hint}"""

        try:
            print("🔄 Calling LLM...")
            response = requests.post(
                self.api_url,
                headers={"Authorization": f"Bearer {OPENAI_API_KEY}"},
                json={
                    "model": OPENAI_MODEL,
                    "messages": [
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    "temperature": LLM_TEMPERATURE
                },
                timeout=LLM_TIMEOUT
            )

            if response.status_code == 200:
                bot_response = response.json()['choices'][0]['message']['content']
                print(f"✅ Got response ({len(bot_response)} chars)")
                return bot_response
            else:
                print(f"❌ LLM error: {response.status_code} {response.text}")
                return "Sorry, I couldn't think of a response right now. 😊"

        except requests.exceptions.Timeout:
            print("⏱️  LLM timeout")
            return "Sorry, that took too long! Let's try again. 😊"
        except requests.exceptions.ConnectionError:
            print(f"❌ Cannot connect to OpenAI API at {LLM_URL}")
            return "Oops! I can't reach my brain right now. 🧠"
        except Exception as e:
            print(f"❌ LLM error: {e}")
            return f"Sorry, something went wrong: {str(e)}"
    
    def _build_system_prompt(self):
        """Construit le prompt système avec personnalité"""
        base_prompt = self.personality.get_system_prompt()
        
        rules_section = ""
        
        english_rule = self.personality.get_rule('english_preference')
        if english_rule:
            rules_section += f"\n\nENGLISH PREFERENCE RULES:\n{english_rule}"
        
        pronunciation_rule = self.personality.get_rule('pronunciation')
        if pronunciation_rule:
            rules_section += f"\n\nPRONUNCIATION RULES:\n{pronunciation_rule}"
        
        engagement_rule = self.personality.get_rule('engagement')
        if engagement_rule:
            rules_section += f"\n\nENGAGEMENT RULES:\n{engagement_rule}"
        
        return base_prompt + rules_section
    
    def _detect_corrections(self, user_text):
        """Détecte si le texte a des erreurs potentielles"""
        hints = ""
        
        if self._is_french(user_text):
            hints += "\n\n⚠️  NOTE: User spoke in French. Gently encourage English and provide translation help."
        
        if self._has_common_errors(user_text):
            hints += "\n⚠️  NOTE: Grammar issue detected. Help gently and constructively."
        
        return hints
    
    def _is_french(self, text):
        """Simple détection français"""
        french_words = [
            'je', 'tu', 'il', 'elle', 'nous', 'vous', 'ils', 'elles',
            'un', 'une', 'des', 'le', 'la', 'les',
            'et', 'ou', 'mais', 'car', 'donc', 'cependant',
            'je suis', "j'ai", "c'est", 'à', 'de', 'pour', 'avec'
        ]
        text_lower = text.lower()
        count = sum(1 for w in french_words if w in text_lower)
        return count >= 2
    
    def _has_common_errors(self, text):
        """Détecte erreurs communes en anglais"""
        import re
        
        errors = [
            r'\bi\s+(?!am|have|\'m|will|can|\'ll|do|like|think)',
            r'\bno\s+(?!one|doubt|way|where)',
        ]
        
        return any(re.search(pattern, text.lower()) for pattern in errors)
    
    def reload_personality(self):
        """Recharge la personnalité (pour live editing)"""
        self.personality.reload()
